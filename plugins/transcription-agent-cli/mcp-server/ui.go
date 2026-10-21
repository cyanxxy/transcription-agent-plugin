package main

import (
	"context"
	_ "embed"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const transcriptViewerURI = "ui://transcription-agent/transcript-v2.html"
const transcriptViewerMIME = "text/html;profile=mcp-app"

// The static viewer contains no recordings, credentials, or transcript data.
// It receives results only from its host's MCP Apps bridge at runtime.
//
//go:embed viewer.html
var transcriptViewerHTML string

func transcriptViewerMeta() mcp.Meta {
	return mcp.Meta{
		"ui":                    map[string]any{"resourceUri": transcriptViewerURI},
		"openai/outputTemplate": transcriptViewerURI,
	}
}

func registerTranscriptViewer(s *mcp.Server) {
	meta := mcp.Meta{
		"ui": map[string]any{
			"prefersBorder": true,
			"csp": map[string]any{
				"connectDomains":  []string{},
				"resourceDomains": []string{},
			},
			"permissions": map[string]any{"clipboardWrite": map[string]any{}},
		},
		"openai/widgetDescription": "An interactive transcript: speaker-colored conversation view with talk-time share and speaker filters, raw file view, search, copy, full-screen mode, and one-click summary, action-item, and decision prompts. Shortened previews are clearly labeled; compatible hosts can load the issued full transcript resource.",
	}
	s.AddResource(&mcp.Resource{
		URI:         transcriptViewerURI,
		Name:        "transcript-viewer",
		Title:       "Transcript viewer",
		Description: "Interactive viewer for a completed transcription result.",
		MIMEType:    transcriptViewerMIME,
		Size:        int64(len(transcriptViewerHTML)),
		Meta:        meta,
	}, func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
			URI:      transcriptViewerURI,
			MIMEType: transcriptViewerMIME,
			Text:     transcriptViewerHTML,
			Meta:     meta,
		}}}, nil
	})
}
