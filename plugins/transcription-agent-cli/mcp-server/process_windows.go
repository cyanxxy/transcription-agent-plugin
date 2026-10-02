//go:build windows

package main

import "os/exec"

func configureCancellation(cmd *exec.Cmd) func() {
	// CommandContext supplies Process.Kill on Windows. The bundled wrapper is
	// POSIX-only; retain a buildable fallback for other MCP distributions.
	return func() {}
}
