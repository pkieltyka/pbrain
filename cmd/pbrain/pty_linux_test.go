//go:build linux

package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"golang.org/x/sys/unix"
)

// openPTY opens a pseudo-terminal with output translation off, so a line feed reaches the reader
// as written; it skips the test where none can be opened.
func openPTY(t *testing.T) (master, slave *os.File) {
	t.Helper()
	fd, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Skipf("no pseudo-terminal: %v", err)
	}
	master = os.NewFile(uintptr(fd), "/dev/ptmx")
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
		master.Close()
		t.Skipf("unlockpt: %v", err)
	}
	n, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	if err != nil {
		master.Close()
		t.Skipf("ptsname: %v", err)
	}
	slave, err = os.OpenFile("/dev/pts/"+strconv.Itoa(n), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		master.Close()
		t.Skipf("open the terminal side: %v", err)
	}
	tio, err := unix.IoctlGetTermios(int(slave.Fd()), unix.TCGETS)
	if err != nil {
		t.Fatal(err)
	}
	tio.Oflag &^= unix.OPOST
	if err := unix.IoctlSetTermios(int(slave.Fd()), unix.TCSETS, tio); err != nil {
		t.Fatal(err)
	}
	return master, slave
}

// pbrain runs the binary as a child with args; its standard output is a pipe, or with tty a terminal.
func pbrain(t *testing.T, tty bool, args ...string) []byte {
	t.Helper()
	cmd := exec.Command(os.Args[0], args...)
	cmd.Args[0] = "pbrain" // registered by testscript.Main in scenario_test.go
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "CLICOLOR_FORCE=1", "FORCE_COLOR=1")
	var diag bytes.Buffer
	cmd.Stderr = &diag
	if !tty {
		out, err := cmd.Output()
		if err != nil || diag.Len() > 0 {
			t.Fatalf("%v: %v %q", args, err, diag.String())
		}
		return out
	}
	master, slave := openPTY(t)
	defer master.Close()
	cmd.Stdout = slave
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	slave.Close()
	var out bytes.Buffer
	done := make(chan struct{})
	go func() { io.Copy(&out, master); close(done) }() // ends with EIO once the child has exited
	if err := cmd.Wait(); err != nil || diag.Len() > 0 {
		t.Fatalf("%v: %v %q", args, err, diag.String())
	}
	<-done
	return out.Bytes()
}

// added matches the sequences pbrain adds: bold, cyan and the reset.
var added = regexp.MustCompile("\x1b\\[(1|36|0)m")

// Scenario 41: search and get print the same bytes to a pipe and to a terminal, with and without
// --color; without it pbrain adds no sequence whatever the terminal and the environment, and with it
// removing the added sequences gives the plain page. A stored escape sequence is printed as is.
func TestColourPipeAndTerminal(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pbrain.toml"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	const id = "20261121-120000-colour-c4c4"
	stored := "plain words, then \x1b[31mred\x1b[39m, then more plain words\n" // none of the added ones
	item := "---\nkind: note\ntitle: Colour check\ncreated: 2026-11-21T12:00:00Z\nauthor: agent:x\n---\n" + stored
	os.MkdirAll(filepath.Join(root, "notes"), 0o755)
	if err := os.WriteFile(filepath.Join(root, "notes", id+".md"), []byte(item), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"search", "plain"}, {"get", id}} {
		base := append([]string{"--brain", root}, args...)
		plain := pbrain(t, false, base...)
		if got := pbrain(t, true, base...); !bytes.Equal(got, plain) {
			t.Fatalf("%v: terminal %q, pipe %q", args, got, plain)
		}
		if args[0] == "get" && !bytes.Contains(plain, []byte(stored)) {
			t.Fatalf("get changed the stored sequence: %q", plain)
		}
		colour := pbrain(t, false, append(base, "--color")...)
		if got := pbrain(t, true, append(base, "--color")...); !bytes.Equal(got, colour) {
			t.Fatalf("%v --color: terminal %q, pipe %q", args, got, colour)
		}
		if bytes.Equal(colour, plain) || !bytes.Equal(added.ReplaceAll(colour, nil), plain) {
			t.Fatalf("%v --color is not the plain page with sequences added: %q", args, colour)
		}
		if added.Match(plain) {
			t.Fatalf("%v: plain output has sequences pbrain adds: %q", args, plain)
		}
		if args[0] == "get" && !bytes.Contains(colour, []byte(stored)) {
			t.Fatalf("get --color changed the stored sequence: %q", colour)
		}
	}
}
