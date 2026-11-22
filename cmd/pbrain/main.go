// Command pbrain keeps a project's knowledge in Markdown files with front matter.
package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/pkieltyka/pbrain/internal/cli"
	"github.com/pkieltyka/pbrain/internal/embed"
	"github.com/pkieltyka/pbrain/internal/embed/bge"
	"github.com/pkieltyka/pbrain/internal/embed/gemma2"
	"github.com/pkieltyka/pbrain/internal/gitx"
	"github.com/pkieltyka/pbrain/internal/sys"
	"golang.org/x/sys/unix"
)

func main() { os.Exit(run()) }

func releaseModels() embed.Registry {
	return embed.NewRegistry(append([]embed.Entry{bge.Entry(), gemma2.Entry()}, gemma2.GGUFEntries()...)...)
}

func run() int { return runWith(cli.Run) }

func runWith(dispatch func(context.Context, cli.Env) int) int {
	stdout, stderr := os.Stdout, os.Stderr
	discard, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "io: could not initialize output streams")
		return 1
	}
	defer discard.Close()
	os.Stdout, os.Stderr = discard, discard

	dir, _ := os.Getwd()
	environ := os.Environ()
	values := make(map[string]string, len(environ))
	for _, entry := range environ {
		if name, value, ok := strings.Cut(entry, "="); ok {
			values[name] = value
		}
	}
	clock := time.Now
	if tz, set := values["TZ"]; set && tz == "" {
		clock = func() time.Time { return time.Now().UTC() }
	}
	gitPath, _ := exec.LookPath("git")
	env := cli.Env{
		Args: os.Args[1:], Stdout: stdout, Stderr: stderr,
		Dir: dir, Now: clock, Rand: rand.Reader, Git: gitx.NewRunner(gitPath, environ),
		Getenv: func(name string) string { return values[name] },
		Files:  sys.Native(),
		// The fetch client supplies the native defaults for a zero Net. The release registry
		// holds the models whose gates pass (semantic.md "Calibration").
		Models: releaseModels(),
	}
	if _, err := unix.IoctlGetTermios(int(os.Stdin.Fd()), terminalRequest); err != nil {
		env.Stdin = os.Stdin
	}
	return dispatch(context.Background(), env)
}
