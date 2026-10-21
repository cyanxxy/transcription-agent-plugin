package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestDiscoverySchemasAndClientMetadata(t *testing.T) {
	cli, _ := fakeCLI(t, "")
	cs := connect(t, cli, nil)
	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.OutputSchema == nil {
			t.Fatalf("%s has no output schema", tool.Name)
		}
		if tool.Annotations == nil || tool.Annotations.DestructiveHint == nil || tool.Annotations.OpenWorldHint == nil {
			t.Fatalf("incomplete annotations: %+v", tool)
		}
		for _, key := range []string{"openai/toolInvocation/invoking", "openai/toolInvocation/invoked"} {
			text, ok := tool.Meta[key].(string)
			if !ok || text == "" || len([]rune(text)) > 64 {
				t.Fatalf("%s: invalid %s=%v", tool.Name, key, tool.Meta[key])
			}
		}
		if tool.Name == "transcribe" {
			schema := tool.InputSchema.(map[string]any)
			if schema["additionalProperties"] != false {
				t.Fatal("unexpected arguments must be rejected")
			}
			properties := schema["properties"].(map[string]any)
			if len(properties["format"].(map[string]any)["enum"].([]any)) != len(formatNames) {
				t.Fatal("format enum missing")
			}
			if properties["max_inline_bytes"].(map[string]any)["default"] != float64(defaultInlineBytes) {
				t.Fatal("preview default missing")
			}
		}
	}
	for _, args := range []map[string]any{
		{"audio_path": "/missing.mp3", "unexpected": true},
		{"audio_path": "/missing.mp3", "max_inline_bytes": maxInlineBytes + 1},
		{"audio_path": "/missing.mp3", "max_inline_bytes": -1},
		{"audio_path": "/missing.mp3", "max_inline_bytes": 1.5},
	} {
		res, _ := call(t, cs, args)
		if !res.IsError || !strings.Contains(errorText(res), "validating") {
			t.Fatalf("schema-invalid input accepted: %v", args)
		}
	}
}

func TestPreviewIsUTF8SafeAndResourceReturnsCompleteTranscript(t *testing.T) {
	text := strings.Repeat("€界🙂", 4000)
	fixture := filepath.Join(t.TempDir(), "transcript.txt")
	if err := os.WriteFile(fixture, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	cli, _ := fakeCLI(t, `cat "`+fixture+`" > "$out"`)
	cs := connect(t, cli, nil)
	res, out := call(t, cs, map[string]any{"audio_path": audioFile(t, "unicode.mp3"), "max_inline_bytes": 5})
	if res.IsError {
		t.Fatal(errorText(res))
	}
	if !utf8.ValidString(out.Transcript) || out.Transcript != "€" || !out.Truncated {
		t.Fatalf("unsafe preview=%q", out.Transcript)
	}
	if out.ResourceURI == "" || out.MIMEType != "text/plain" {
		t.Fatalf("artifact missing: %+v", out)
	}
	var link *mcp.ResourceLink
	for _, content := range res.Content {
		if r, ok := content.(*mcp.ResourceLink); ok {
			link = r
		}
	}
	if link == nil || link.URI != out.ResourceURI || link.Size == nil || *link.Size != int64(len(text)) {
		t.Fatalf("resource link=%+v", link)
	}
	full, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: out.ResourceURI})
	if err != nil {
		t.Fatal(err)
	}
	if len(full.Contents) != 1 || full.Contents[0].Text != text || full.Contents[0].URI != out.ResourceURI {
		t.Fatal("resource lost transcript content")
	}
	for _, uri := range []string{"transcriber://transcripts/unissued", "file://" + fixture, "transcriber://transcripts/../../" + fixture} {
		if _, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: uri}); err == nil {
			t.Fatalf("unissued resource accepted: %s", uri)
		}
	}
}

func TestPreviewLimitZeroAndOverride(t *testing.T) {
	cli, _ := fakeCLI(t, `head -c 150000 /dev/zero | tr '\0' x > "$out"`)
	cs := connect(t, cli, nil)
	for _, limit := range []int{0, maxInlineBytes} {
		res, out := call(t, cs, map[string]any{"audio_path": audioFile(t, "long.mp3"), "max_inline_bytes": limit})
		if res.IsError {
			t.Fatal(errorText(res))
		}
		if len(out.Transcript) != limit || !out.Truncated || out.Bytes != 150000 {
			t.Fatalf("limit=%d out=%+v", limit, out)
		}
	}
}

func TestIssuedResourceRejectsReplacedAndEditedFile(t *testing.T) {
	cli, _ := fakeCLI(t, `echo original > "$out"`)
	for _, replace := range []bool{false, true} {
		cs := connect(t, cli, nil)
		res, out := call(t, cs, map[string]any{"audio_path": audioFile(t, "original.mp3")})
		if res.IsError {
			t.Fatal(errorText(res))
		}
		if replace {
			private := filepath.Join(t.TempDir(), "private.txt")
			if err := os.WriteFile(private, []byte("unrelated private data"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(out.OutputPath); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(private, out.OutputPath); err != nil {
				t.Fatal(err)
			}
		} else if err := os.WriteFile(out.OutputPath, []byte("edited file"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: out.ResourceURI}); err == nil {
			t.Fatal("changed artifact was read")
		}
	}
}

func TestResourceReadLimit(t *testing.T) {
	cli, _ := fakeCLI(t, `dd if=/dev/zero of="$out" bs=1048576 count=17 2>/dev/null`)
	cs := connect(t, cli, nil)
	res, out := call(t, cs, map[string]any{"audio_path": audioFile(t, "large.mp3"), "max_inline_bytes": 0})
	if res.IsError {
		t.Fatal(errorText(res))
	}
	if _, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: out.ResourceURI}); err == nil || !strings.Contains(err.Error(), "resource limit") {
		t.Fatalf("large resource error=%v", err)
	}
	if info, err := os.Stat(out.OutputPath); err != nil || info.Size() != 17*1048576 {
		t.Fatalf("complete file unavailable: %v", err)
	}
}

func TestStructuredRecoverableFailure(t *testing.T) {
	cli, _ := fakeCLI(t, `echo "gemini: not configured" >&2; exit 1`)
	res, _ := call(t, connect(t, cli, nil), map[string]any{"audio_path": audioFile(t, "failure.mp3")})
	if !res.IsError {
		t.Fatal("expected provider failure")
	}
	data, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var out TranscribeOut
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if out.Error == nil || out.Error.Code != "provider_not_configured" || out.Error.Retryable || !strings.Contains(out.Error.NextAction, "auth_status") {
		t.Fatalf("unrecoverable failure=%s", data)
	}
	if strings.Contains(string(data), "output_path") {
		t.Fatal("failure pretends to contain an artifact")
	}
}

func TestAuthStatusIncludesProviderAndDependencyStates(t *testing.T) {
	cli := filepath.Join(t.TempDir(), "cli")
	if err := os.WriteFile(cli, []byte("#!/bin/bash\nprintf 'gemini: configured via environment\\nmeta: not configured\\nmicrosoft: incomplete (needs endpoint)\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "ffmpeg"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	res, err := connect(t, cli, nil).CallTool(context.Background(), &mcp.CallToolParams{Name: "auth_status", Arguments: map[string]any{}})
	if err != nil || res.IsError {
		t.Fatalf("status failed: %v", err)
	}
	data, _ := json.Marshal(res.StructuredContent)
	var status StatusOut
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatal(err)
	}
	if len(status.Providers) != 3 || status.Providers[0].State != "configured" || status.Providers[0].Source != "environment" || status.Providers[1].State != "not_configured" || status.Providers[2].State != "incomplete" {
		t.Fatalf("providers=%+v", status.Providers)
	}
	if len(status.Dependencies) != 2 || !status.Dependencies[0].Available || status.Dependencies[1].Available {
		t.Fatalf("dependencies=%+v", status.Dependencies)
	}
}

func TestRunCancellationStopsDescendants(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "orphan")
	cli, _ := fakeCLI(t, `(sleep 1; echo orphan > "`+marker+`") & wait`)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := run(ctx, cli, "ignored")
	if err == nil || time.Since(start) > time.Second {
		t.Fatalf("cancellation did not promptly stop process group: %v", err)
	}
	time.Sleep(1100 * time.Millisecond)
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("CLI descendant survived cancellation")
	}
}

func TestRunDiagnosticOutputIsBoundedAndUTF8Safe(t *testing.T) {
	cli, _ := fakeCLI(t, `head -c 200000 /dev/zero | tr '\0' x; printf '€界🙂'; exit 1`)
	out, err := run(context.Background(), cli, "ignored")
	if err == nil || len(out) > 16384 || !utf8.ValidString(out) || !strings.HasSuffix(out, "€界🙂") {
		t.Fatalf("unbounded output: len=%d err=%v", len(out), err)
	}
	diagnostic := commandError("failed", out, err).Error()
	if len(diagnostic) > 2100 || !utf8.ValidString(diagnostic) {
		t.Fatalf("unsafe diagnostic: len=%d", len(diagnostic))
	}
}

func TestLatestAndLegacySDKRoundTrips(t *testing.T) {
	for _, revision := range []string{"2025-06-18", "2025-11-25", "2026-07-28"} {
		t.Run(revision, func(t *testing.T) {
			cli, _ := fakeCLI(t, `echo interoperable > "$out"`)
			ctx := context.Background()
			st, ct := mcp.NewInMemoryTransports()
			if _, err := newServer(cli).Connect(ctx, st, nil); err != nil {
				t.Fatal(err)
			}
			cs, err := mcp.NewClient(&mcp.Implementation{Name: "version-test", Version: "1"}, nil).Connect(ctx, ct, &mcp.ClientSessionOptions{ProtocolVersion: revision})
			if err != nil {
				t.Fatal(err)
			}
			defer cs.Close()
			if cs.InitializeResult().ProtocolVersion != revision {
				t.Fatalf("negotiated=%s", cs.InitializeResult().ProtocolVersion)
			}
			res, out := call(t, cs, map[string]any{"audio_path": audioFile(t, "interop.mp3")})
			if res.IsError || out.Transcript != "interoperable\n" {
				t.Fatalf("call failed: %s", errorText(res))
			}
			full, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: out.ResourceURI})
			if err != nil || len(full.Contents) != 1 || full.Contents[0].Text != out.Transcript {
				t.Fatalf("resource roundtrip: %v", err)
			}
			if revision == "2026-07-28" && (full.CacheScope != "private" || full.TTLMs != 0) {
				t.Fatalf("private transcript cache=%+v", full.Cacheable)
			}
		})
	}
}

func TestPublishedIdentityCannotBindToReplacement(t *testing.T) {
	for _, method := range []string{"link", "rename", "copy"} {
		t.Run(method, func(t *testing.T) {
			dir := t.TempDir()
			temp, dest, unrelated := filepath.Join(dir, "temp.txt"), filepath.Join(dir, "dest.txt"), filepath.Join(dir, "private.txt")
			if err := os.WriteFile(temp, []byte("completed transcript"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(unrelated, []byte("unrelated private data"), 0o600); err != nil {
				t.Fatal(err)
			}
			link := os.Link
			if method == "copy" {
				link = func(string, string) error { return errors.New("hard links unavailable") }
			}
			published, err := publishWithLink(temp, dest, method == "rename", link)
			if err != nil {
				t.Fatal(err)
			}
			actual, err := os.Stat(dest)
			if err != nil || !os.SameFile(published, actual) {
				t.Fatalf("returned identity is not the published file: %v", err)
			}
			if method == "copy" {
				source, _ := os.Stat(temp)
				if os.SameFile(published, source) {
					t.Fatal("copy identity incorrectly uses source inode")
				}
			}
			// Deterministically simulate replacement between publication and URI
			// issuance. The URI must remain tied to the descriptor's identity.
			if err := os.Remove(dest); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(unrelated, dest); err != nil {
				t.Fatal(err)
			}
			store := newArtifactStore()
			uri := store.issue(dest, "txt", published, nil).URI
			result, err := store.read(context.Background(), &mcp.ReadResourceRequest{Params: &mcp.ReadResourceParams{URI: uri}})
			if err == nil || result != nil {
				t.Fatal("resource disclosed replacement file")
			}
		})
	}
}

func TestArtifactStoreEvictsOldestWithoutDeletingFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "transcript.txt")
	if err := os.WriteFile(path, []byte("kept locally"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	store := newArtifactStore()
	first := store.issue(path, "txt", info, nil).URI
	var latest string
	for range maxIssuedArtifacts {
		latest = store.issue(path, "txt", info, nil).URI
	}
	if len(store.issued) != maxIssuedArtifacts || len(store.order) != maxIssuedArtifacts {
		t.Fatal("artifact index grew beyond bound")
	}
	if _, err := store.read(context.Background(), &mcp.ReadResourceRequest{Params: &mcp.ReadResourceParams{URI: first}}); err == nil {
		t.Fatal("oldest URI was not evicted")
	}
	if result, err := store.read(context.Background(), &mcp.ReadResourceRequest{Params: &mcp.ReadResourceParams{URI: latest}}); err != nil || result.Contents[0].Text != "kept locally" {
		t.Fatalf("latest artifact unavailable: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("eviction deleted transcript file")
	}
}

func TestProviderStatusMixedMicrosoftSources(t *testing.T) {
	providers := providerStatuses("microsoft: configured (key via environment, endpoint via local file)")
	if providers[2].State != "configured" || providers[2].Source != "(key via environment, endpoint via local file)" {
		t.Fatalf("mixed source status=%+v", providers[2])
	}
}

func TestOlderProtocolNegotiatesSupportedRevision(t *testing.T) {
	cli, _ := fakeCLI(t, "")
	st, ct := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := newServer(cli).Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "older-version-test", Version: "1"}, nil).Connect(ctx, ct, &mcp.ClientSessionOptions{ProtocolVersion: "2024-11-05"})
	if err != nil {
		return
	} // A client unable to renegotiate must refuse connection.
	defer cs.Close()
	if cs.InitializeResult().ProtocolVersion < "2025-06-18" {
		t.Fatalf("unsupported legacy version accepted: %s", cs.InitializeResult().ProtocolVersion)
	}
}

func TestMalformedUTF8IsRejectedAtPreviewBoundary(t *testing.T) {
	for _, fixture := range []struct {
		data  []byte
		limit int
	}{
		{[]byte{'a', 0xff}, 2},
		{[]byte{0xe2, 0x82}, 1},
		{[]byte{0xff, 'a'}, 0},
	} {
		path := filepath.Join(t.TempDir(), "malformed.txt")
		if err := os.WriteFile(path, fixture.data, 0o600); err != nil {
			t.Fatal(err)
		}
		cli, _ := fakeCLI(t, `cat "`+path+`" > "$out"`)
		res, _ := call(t, connect(t, cli, nil), map[string]any{"audio_path": audioFile(t, "bad.mp3"), "max_inline_bytes": fixture.limit})
		if !res.IsError || !strings.Contains(errorText(res), "not valid UTF-8") {
			t.Fatalf("malformed output silently trimmed: %q", fixture.data)
		}
	}
}

func TestInvalidDiagnosticBytesStayWithinLimit(t *testing.T) {
	cli, _ := fakeCLI(t, `printf '\377%.0s' {1..20000}; exit 1`)
	out, err := run(context.Background(), cli, "ignored")
	if err == nil || len(out) > 16384 || !utf8.ValidString(out) {
		t.Fatalf("invalid diagnostics expanded beyond limit: len=%d err=%v", len(out), err)
	}
}
