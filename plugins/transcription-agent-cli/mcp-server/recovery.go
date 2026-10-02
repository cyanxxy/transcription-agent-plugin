package main

import (
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Once the provider has finished, a late output collision must not discard
// billed work. Preserve it at a unique path without replacing the destination.
func recoverTranscription(cause error, temp string, originalInfo os.FileInfo, in TranscribeIn, session *mcp.ServerSession, artifacts *artifactStore) (*mcp.CallToolResult, TranscribeOut, error) {
	result, out, _ := failedTranscribe("output_conflict", cause, "The transcription completed but could not be published. Check the destination before paying for another run.")
	ext := filepath.Ext(in.OutputPath)
	stem := strings.TrimSuffix(filepath.Base(in.OutputPath), ext)
	recovered := filepath.Join(filepath.Dir(in.OutputPath), "."+stem+".recovered-"+rand.Text()+ext)
	info, err := publishWithLinkMode(temp, recovered, false, os.Link, 0o600)
	if err != nil {
		// The completed source already exists even when a second name cannot
		// be created. Bind its original identity and tell the caller to keep it.
		recovered, info = temp, originalInfo
	}
	// Recovery artifacts are private. The FileInfo identity was captured from
	// a descriptor by publish, and does not rely on a later path lookup.
	resource := artifacts.issue(recovered, in.Format, info, session)
	out.Error.RecoveryOutputPath = recovered
	out.Error.ResourceURI = resource.URI
	out.Error.NextAction = "Read recovery_output_path or fetch resource_uri for the completed transcript. Move it to a chosen destination without starting another billed transcription."
	data, _ := json.Marshal(out)
	result.Content = []mcp.Content{&mcp.TextContent{Text: string(data)}, resource}
	return result, out, nil
}
