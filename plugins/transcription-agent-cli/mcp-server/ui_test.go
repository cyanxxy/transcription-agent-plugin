package main

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestTranscriptViewerResource(t *testing.T) {
	cli, _ := fakeCLI(t, "")
	cs := connect(t, cli, nil)
	resources, err := cs.ListResources(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, resource := range resources.Resources {
		if resource.URI == transcriptViewerURI {
			found = resource.MIMEType == transcriptViewerMIME
		}
	}
	if !found {
		t.Fatal("MCP Apps viewer resource was not advertised")
	}
	result, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: transcriptViewerURI})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Contents) != 1 || result.Contents[0].URI != transcriptViewerURI || result.Contents[0].MIMEType != transcriptViewerMIME {
		t.Fatalf("unexpected viewer result: %+v", result)
	}
	if result.Contents[0].Text != transcriptViewerHTML || !strings.Contains(transcriptViewerHTML, "ui/initialize") {
		t.Fatal("viewer content or bridge missing")
	}
	ui, ok := result.Contents[0].Meta["ui"].(map[string]any)
	if !ok || ui["csp"] == nil || ui["permissions"] == nil {
		t.Fatal("viewer CSP and clipboard permission metadata missing")
	}
	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name == "transcribe" {
			ui, ok := tool.Meta["ui"].(map[string]any)
			if !ok || ui["resourceUri"] != transcriptViewerURI {
				t.Fatal("transcribe does not point at the viewer")
			}
		}
	}
}
