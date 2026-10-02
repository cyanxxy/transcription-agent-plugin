package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// fakeCLI writes a stand-in for the transcriber-cli wrapper. It records its
// arguments and credential variables, then writes a transcript to -o.
func fakeCLI(t *testing.T, body string) (cli, argsFile string) {
	t.Helper()
	dir := t.TempDir()
	argsFile = filepath.Join(dir, "args")
	cli = filepath.Join(dir, "transcriber-cli")
	script := `#!/usr/bin/env bash
printf '%s\n' "$@" > "` + argsFile + `"
env | grep -E '^(GEMINI_API_KEY|META_API_KEY)=' >> "` + argsFile + `" || true
if [[ "$1" == auth ]]; then echo "gemini: configured via local file"; exit 0; fi
while (($#)); do [[ "$1" == -o ]] && out="$2"; shift; done
` + body
	if err := os.WriteFile(cli, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return cli, argsFile
}

func connect(t *testing.T, cli string, opts *mcp.ClientOptions) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	if _, err := newServer(cli).Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, opts).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func audioFile(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte("fake audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func call(t *testing.T, cs *mcp.ClientSession, args map[string]any) (*mcp.CallToolResult, TranscribeOut) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "transcribe", Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	var out TranscribeOut
	if !res.IsError {
		b, _ := json.Marshal(res.StructuredContent)
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatal(err)
		}
	}
	return res, out
}

func errorText(res *mcp.CallToolResult) string {
	var parts []string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func TestListsTools(t *testing.T) {
	cli, _ := fakeCLI(t, "")
	res, err := connect(t, cli, nil).ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}
	if strings.Join(names, ",") != "auth_status,transcribe" {
		t.Fatalf("tools = %v", names)
	}
}

func TestTranscribeDefaults(t *testing.T) {
	cli, argsFile := fakeCLI(t, `printf 'Speaker 1: hello\n' > "$out"`)
	audio := audioFile(t, "meeting.m4a")
	res, out := call(t, connect(t, cli, nil), map[string]any{"audio_path": audio})
	if res.IsError {
		t.Fatalf("unexpected error: %s", errorText(res))
	}
	want := strings.TrimSuffix(audio, ".m4a") + ".txt"
	if out.OutputPath != want || out.Format != "txt" || out.Model != "gemini-3.5-transcribe" {
		t.Fatalf("out = %+v", out)
	}
	if out.Transcript != "Speaker 1: hello\n" || out.Truncated || out.Bytes != 17 {
		t.Fatalf("transcript = %+v", out)
	}
	args, _ := os.ReadFile(argsFile)
	// The engine writes a temporary file beside the destination.
	got := strings.Fields(string(args))
	temp := got[5]
	if strings.Join(got[:5], " ") != "-model gemini-3.5-transcribe -i "+audio+" -o" || strings.Join(got[6:8], " ") != "-format txt" {
		t.Fatalf("args = %q", got)
	}
	if filepath.Dir(temp) != filepath.Dir(want) || !strings.HasPrefix(filepath.Base(temp), ".meeting.") || !strings.HasSuffix(temp, ".partial.txt") {
		t.Fatalf("temporary output = %q", temp)
	}
	if _, err := os.Stat(temp); err == nil {
		t.Fatal("temporary output was left behind")
	}
}

func TestTranscribeFormatAndModel(t *testing.T) {
	cli, argsFile := fakeCLI(t, `echo 1 > "$out"`)
	audio := audioFile(t, "call.WAV")
	res, out := call(t, connect(t, cli, nil), map[string]any{
		"audio_path": audio, "format": "srt", "model": "muse-voice-transcribe-1.0",
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", errorText(res))
	}
	if !strings.HasSuffix(out.OutputPath, "call.srt") || out.Model != "muse-voice-transcribe-1.0" {
		t.Fatalf("out = %+v", out)
	}
	if args, _ := os.ReadFile(argsFile); !strings.Contains(string(args), "muse-voice-transcribe-1.0\n") {
		t.Fatalf("args = %s", args)
	}
}

func TestTranscribeRejectsBadArguments(t *testing.T) {
	cli, argsFile := fakeCLI(t, `echo 1 > "$out"`)
	cs := connect(t, cli, nil)
	audio := audioFile(t, "memo.ogg")
	existing := strings.TrimSuffix(audio, ".ogg") + ".txt"
	if err := os.WriteFile(existing, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{"missing path", map[string]any{}, "audio_path"},
		{"blank path", map[string]any{"audio_path": " "}, "audio_path is required"},
		{"relative path", map[string]any{"audio_path": "memo.ogg"}, "must be an absolute path"},
		{"missing file", map[string]any{"audio_path": filepath.Join(t.TempDir(), "none.mp3")}, "cannot read audio_path"},
		{"not audio", map[string]any{"audio_path": audioFile(t, "notes.md")}, "must be an MP3"},
		{"unknown format", map[string]any{"audio_path": audio, "format": "vtt"}, "/properties/format"},
		{"unknown model", map[string]any{"audio_path": audio, "model": "whisper-1"}, "/properties/model"},
		{"existing output", map[string]any{"audio_path": audio}, "already exists"},
		{"wrong extension", map[string]any{"audio_path": audio, "output_path": filepath.Join(t.TempDir(), ".zshrc")}, "must end in .txt"},
	}
	for _, tc := range cases {
		res, _ := call(t, cs, tc.args)
		if !res.IsError || !strings.Contains(errorText(res), tc.want) {
			t.Errorf("%s: IsError=%v text=%q, want %q", tc.name, res.IsError, errorText(res), tc.want)
		}
	}
	if _, err := os.Stat(argsFile); err == nil {
		t.Error("the CLI ran for rejected arguments")
	}
	if b, _ := os.ReadFile(existing); string(b) != "keep" {
		t.Error("existing output was replaced")
	}
	res, _ := call(t, cs, map[string]any{"audio_path": audio, "overwrite": true})
	if res.IsError {
		t.Fatalf("overwrite: %s", errorText(res))
	}
}

// leftovers lists temporary transcript files the server left in dir.
func leftovers(t *testing.T, dir string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".*.partial.*"))
	if err != nil {
		t.Fatal(err)
	}
	return matches
}

func TestTranscribeKeepsFileCreatedDuringRun(t *testing.T) {
	cli, _ := fakeCLI(t, `sleep 0.3; echo new > "$out"`)
	cs := connect(t, cli, nil)
	audio := audioFile(t, "meeting.mp3")
	dest := strings.TrimSuffix(audio, ".mp3") + ".txt"
	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = os.WriteFile(dest, []byte("mine"), 0o644)
	}()
	res, _ := call(t, cs, map[string]any{"audio_path": audio})
	if !res.IsError || !strings.Contains(errorText(res), "already exists") {
		t.Fatalf("IsError=%v text=%q", res.IsError, errorText(res))
	}
	if b, _ := os.ReadFile(dest); string(b) != "mine" {
		t.Fatalf("file created during the run was replaced: %q", b)
	}
	data, _ := json.Marshal(res.StructuredContent)
	var recovery TranscribeOut
	if err := json.Unmarshal(data, &recovery); err != nil {
		t.Fatal(err)
	}
	if recovery.Error == nil || recovery.Error.RecoveryOutputPath == "" || recovery.Error.ResourceURI == "" {
		t.Fatalf("paid transcript was not preserved: %s", data)
	}
	if b, _ := os.ReadFile(recovery.Error.RecoveryOutputPath); string(b) != "new\n" {
		t.Fatalf("recovered transcript=%q", b)
	}
	if info, err := os.Stat(recovery.Error.RecoveryOutputPath); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("recovery permissions: %v", err)
	}
	full, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: recovery.Error.ResourceURI})
	if err != nil || len(full.Contents) != 1 || full.Contents[0].Text != "new\n" {
		t.Fatalf("recovery resource unavailable: %v", err)
	}
	if l := leftovers(t, filepath.Dir(audio)); len(l) > 0 {
		t.Fatalf("temporary files left: %v", l)
	}
}

func TestTranscribeConcurrentCallsWriteOnce(t *testing.T) {
	counter := filepath.Join(t.TempDir(), "runs")
	cli, _ := fakeCLI(t, `echo run >> "`+counter+`"; sleep 0.2; echo "$RANDOM" > "$out"`)
	cs := connect(t, cli, nil)
	audio := audioFile(t, "meeting.mp3")
	var wg sync.WaitGroup
	var mu sync.Mutex
	var ok, exists int
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "transcribe", Arguments: map[string]any{"audio_path": audio}})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil && !res.IsError:
				ok++
			case err == nil && (strings.Contains(errorText(res), "already exists") || strings.Contains(errorText(res), "already running")):
				exists++
			default:
				t.Errorf("unexpected result: %v %q", err, errorText(res))
			}
		}()
	}
	wg.Wait()
	if ok != 1 || exists != 3 {
		t.Fatalf("successes=%d already-exists=%d, want 1 and 3", ok, exists)
	}
	if data, _ := os.ReadFile(counter); string(data) != "run\n" {
		t.Fatalf("duplicate billed jobs were started: %q", data)
	}
	if l := leftovers(t, filepath.Dir(audio)); len(l) > 0 {
		t.Fatalf("temporary files left: %v", l)
	}
}

func TestTranscribeOverwriteReplacesAndCleansUp(t *testing.T) {
	cli, _ := fakeCLI(t, `echo new > "$out"`)
	audio := audioFile(t, "meeting.mp3")
	dest := strings.TrimSuffix(audio, ".mp3") + ".txt"
	if err := os.WriteFile(dest, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, out := call(t, connect(t, cli, nil), map[string]any{"audio_path": audio, "overwrite": true})
	if res.IsError {
		t.Fatal(errorText(res))
	}
	if b, _ := os.ReadFile(dest); string(b) != "new\n" || out.Transcript != "new\n" {
		t.Fatalf("dest=%q transcript=%q", b, out.Transcript)
	}
	if info, _ := os.Stat(dest); info.Mode().Perm() != 0o644 {
		t.Fatalf("mode = %v", info.Mode().Perm())
	}
	if l := leftovers(t, filepath.Dir(audio)); len(l) > 0 {
		t.Fatalf("temporary files left: %v", l)
	}
}

func TestFailedRunLeavesNoFiles(t *testing.T) {
	cli, _ := fakeCLI(t, `echo partial > "$out"; exit 1`)
	audio := audioFile(t, "meeting.mp3")
	res, _ := call(t, connect(t, cli, nil), map[string]any{"audio_path": audio})
	if !res.IsError {
		t.Fatal("expected an error")
	}
	if _, err := os.Stat(strings.TrimSuffix(audio, ".mp3") + ".txt"); err == nil {
		t.Fatal("a failed run published a file")
	}
	if l := leftovers(t, filepath.Dir(audio)); len(l) > 0 {
		t.Fatalf("temporary files left: %v", l)
	}
}

func TestDestructiveHint(t *testing.T) {
	cli, _ := fakeCLI(t, "")
	res, err := connect(t, cli, nil).ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		if tool.Name == "transcribe" && (tool.Annotations.DestructiveHint == nil || !*tool.Annotations.DestructiveHint) {
			t.Fatal("transcribe must not claim to be non-destructive")
		}
	}
}

func TestTranscribeReportsCLIFailure(t *testing.T) {
	cli, _ := fakeCLI(t, `echo "transcriber-cli: gemini: not configured" >&2; exit 1`)
	res, _ := call(t, connect(t, cli, nil), map[string]any{"audio_path": audioFile(t, "a.mp3")})
	if !res.IsError || !strings.Contains(errorText(res), "gemini: not configured") {
		t.Fatalf("result = %v %q", res.IsError, errorText(res))
	}
}

func TestTranscribeTruncatesLongTranscripts(t *testing.T) {
	cli, _ := fakeCLI(t, `head -c 150000 /dev/zero | tr '\0' x > "$out"`)
	res, out := call(t, connect(t, cli, nil), map[string]any{"audio_path": audioFile(t, "a.flac")})
	if res.IsError {
		t.Fatal(errorText(res))
	}
	if !out.Truncated || len(out.Transcript) != defaultInlineBytes || out.Bytes != 150000 {
		t.Fatalf("truncated=%v len=%d bytes=%d", out.Truncated, len(out.Transcript), out.Bytes)
	}
}

func TestTranscribeSendsProgress(t *testing.T) {
	old := progressInterval
	progressInterval = 20 * time.Millisecond
	t.Cleanup(func() { progressInterval = old })
	cli, _ := fakeCLI(t, `sleep 0.2; echo done > "$out"`)
	var mu sync.Mutex
	var got []float64
	cs := connect(t, cli, &mcp.ClientOptions{
		ProgressNotificationHandler: func(_ context.Context, req *mcp.ProgressNotificationClientRequest) {
			mu.Lock()
			got = append(got, req.Params.Progress)
			mu.Unlock()
		},
	})
	params := &mcp.CallToolParams{Name: "transcribe", Arguments: map[string]any{"audio_path": audioFile(t, "a.mp3")}}
	params.SetProgressToken("tok")
	res, err := cs.CallTool(context.Background(), params)
	if err != nil || res.IsError {
		t.Fatalf("err=%v result=%q", err, errorText(res))
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) == 0 {
		t.Fatal("no progress notifications")
	}
	for i := 1; i < len(got); i++ {
		if got[i] <= got[i-1] {
			t.Fatalf("progress went backwards: %v", got)
		}
	}
}

func TestTranscribeCancelStopsCLI(t *testing.T) {
	cli, _ := fakeCLI(t, `sleep 30; echo late > "$out"`)
	cs := connect(t, cli, nil)
	audio := audioFile(t, "a.mp3")
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "transcribe", Arguments: map[string]any{"audio_path": audio}})
	if err == nil {
		t.Fatal("expected the call to be cancelled")
	}
	time.Sleep(500 * time.Millisecond)
	if _, err := os.Stat(strings.TrimSuffix(audio, ".mp3") + ".txt"); err == nil {
		t.Fatal("cancelled run still wrote output")
	}
	if time.Since(start) > 15*time.Second {
		t.Fatal("cancel took too long")
	}
	if l := leftovers(t, filepath.Dir(audio)); len(l) > 0 {
		t.Fatalf("cancelled transcription left temporary output: %v", l)
	}
}

func TestAuthStatus(t *testing.T) {
	cli, _ := fakeCLI(t, "")
	res, err := connect(t, cli, nil).CallTool(context.Background(), &mcp.CallToolParams{Name: "auth_status", Arguments: map[string]any{}})
	if err != nil || res.IsError {
		t.Fatalf("err=%v result=%q", err, errorText(res))
	}
	b, _ := json.Marshal(res.StructuredContent)
	if !strings.Contains(string(b), "gemini: configured via local file") {
		t.Fatalf("structured = %s", b)
	}
}

func TestUnfilledCredentialsAreDropped(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "${user_config.gemini_api_key}")
	t.Setenv("META_API_KEY", "m-key")
	cli, argsFile := fakeCLI(t, `echo 1 > "$out"`)
	res, _ := call(t, connect(t, cli, nil), map[string]any{"audio_path": audioFile(t, "a.mp3")})
	if res.IsError {
		t.Fatal(errorText(res))
	}
	args, _ := os.ReadFile(argsFile)
	if strings.Contains(string(args), "GEMINI_API_KEY") || !strings.Contains(string(args), "META_API_KEY=m-key") {
		t.Fatalf("child env = %s", args)
	}
}

func TestFindCLI(t *testing.T) {
	t.Setenv("TRANSCRIBER_CLI", "relative/cli")
	if _, err := findCLI(); err == nil {
		t.Error("relative TRANSCRIBER_CLI accepted")
	}
	t.Setenv("TRANSCRIBER_CLI", "/opt/x/transcriber-cli")
	if p, err := findCLI(); err != nil || p != "/opt/x/transcriber-cli" {
		t.Errorf("TRANSCRIBER_CLI: %q %v", p, err)
	}
	home := t.TempDir()
	t.Setenv("TRANSCRIBER_CLI", "${user_config.cli_path}")
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())
	if _, err := findCLI(); err == nil {
		t.Error("found a CLI that does not exist")
	}
	cli := filepath.Join(home, ".local", "bin", "transcriber-cli")
	if err := os.MkdirAll(filepath.Dir(cli), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cli, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if p, err := findCLI(); err != nil || p != cli {
		t.Errorf("fallback: %q %v", p, err)
	}
}
