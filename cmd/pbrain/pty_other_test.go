//go:build !linux

package main

import (
	"os/exec"
	"testing"
)

// ptySkipReason is why the terminal checks of scenario 41 do not run here: the pseudo-terminal code
// is Linux only, and the macOS run is an item of the Mac backlog.
const ptySkipReason = "pty checks run on Linux only; macOS tracked in the Mac backlog"

func ptyAvailable() bool { return false }

// ptyUnavailable is the reason a script's [pty] lines are skipped on this machine.
func ptyUnavailable() string { return ptySkipReason }

// runOnPTY has no pseudo-terminal to offer here: the caller skips.
func runOnPTY(*exec.Cmd) (out []byte, skip string, err error) {
	return nil, ptySkipReason, nil
}

// TestColourPipeAndTerminal is scenario 41's pipe-against-terminal check; here it is skipped by name.
func TestColourPipeAndTerminal(t *testing.T) {
	t.Skip(ptySkipReason)
}
