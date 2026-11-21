package main

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/rogpeppe/go-internal/testscript"
)

// audience-race leaves a real writer waiting for its stdin body while a
// second process changes its audience rule. The destination notice is the
// synchronization point after the first process has selected the old rule.
func scenarioAudienceRace(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 2 {
		ts.Fatalf("usage: audience-race MATCH SOURCE")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	exe, err := os.Executable()
	ts.Check(err)
	child := exec.CommandContext(ctx, exe, "add", "--title", "Racing write", "--source", args[1], "--json")
	child.Args[0], child.Dir = "pbrain-test", ts.MkAbs(".")
	for _, name := range []string{"PATH", "HOME", "GIT_CONFIG_NOSYSTEM", "GIT_CONFIG_GLOBAL", "PBRAIN_AUTHOR", "PBRAIN_TEST_NOW", "TZ", "GOMAXPROCS"} {
		child.Env = append(child.Env, name+"="+ts.Getenv(name))
	}
	stdin, err := child.StdinPipe()
	ts.Check(err)
	defer stdin.Close()
	diag, err := child.StderrPipe()
	ts.Check(err)
	ts.Check(child.Start())
	defer func() {
		if child.ProcessState == nil {
			cancel()
			_ = child.Wait()
		}
	}()
	reader := bufio.NewReader(diag)
	first, err := reader.ReadString('\n')
	ts.Check(err)
	if !strings.Contains(first, `"code":"destination"`) || !strings.Contains(first, "Writing to personal") {
		ts.Fatalf("expected the child to select the old personal rule, got %s", first)
	}
	ts.Check(ts.Exec("pbrain-test", "audience", "set", args[0], "shared"))
	_, err = io.WriteString(stdin, "rule race body\n")
	ts.Check(err)
	ts.Check(stdin.Close())
	remaining, err := io.ReadAll(reader)
	ts.Check(err)
	var exit *exec.ExitError
	if err := child.Wait(); !errors.As(err, &exit) || exit.ExitCode() != 1 || !strings.Contains(string(remaining), `"code":"conflict"`) {
		ts.Fatalf("routed write returned %v, diagnostics %s", err, remaining)
	}
}
