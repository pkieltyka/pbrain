//go:build !linux && !darwin

package main

import (
	"os/exec"
	"testing"
)

// ptySkipReason records which platforms implement the terminal checks.
const ptySkipReason = "pty checks run on Linux and Darwin"

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
