package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/pkieltyka/pbrain/internal/sys"
	"github.com/rogpeppe/go-internal/testscript"
)

// The faults are on the same handle operations as the production writer.
// The kill happens after the final link succeeds, before cleanup or stdout.
type scenarioFiles struct {
	sys.Files
	mode string
}
type scenarioDir struct {
	sys.Dir
	name, mode string
}

func (f *scenarioFiles) OpenRoot(path string, create bool) (sys.Dir, error) {
	d, err := f.Files.OpenRoot(path, create)
	if err != nil {
		return nil, err
	}
	return &scenarioDir{d, filepath.Base(path), f.mode}, nil
}
func (d *scenarioDir) Sub(name string, create bool) (sys.Dir, error) {
	child, err := d.Dir.Sub(name, create)
	if err != nil {
		return nil, err
	}
	return &scenarioDir{child, name, d.mode}, nil
}
func (d *scenarioDir) LinkNew(temp, name string) error {
	if d.mode == "original" && d.name == "originals" {
		return errors.New("scenario original publication failure")
	}
	err := d.Dir.LinkNew(temp, name)
	killItem := d.mode == "kill" && strings.HasSuffix(name, ".md")
	killIndex := d.mode == "kill-index" && d.name == "index" && strings.HasSuffix(name, ".db")
	if err == nil && (killItem || killIndex) {
		return syscall.Kill(os.Getpid(), syscall.SIGKILL)
	}
	return err
}
func (d *scenarioDir) Remove(name string) error {
	if d.mode == "remove-original" && d.name == "originals" {
		return errors.New("scenario original removal failure")
	}
	return d.Dir.Remove(name)
}
func (d *scenarioDir) Sync() error {
	if d.mode == "after" && d.name == "notes" {
		return errors.New("scenario post-publication sync failure")
	}
	return d.Dir.Sync()
}
func unwrapDir(d sys.Dir) sys.Dir { return d.(*scenarioDir).Dir }
func (f *scenarioFiles) DirInfo(d sys.Dir) (sys.FileInfo, error) {
	return f.Files.DirInfo(unwrapDir(d))
}
func (f *scenarioFiles) Entries(d sys.Dir) ([]sys.Entry, error) { return f.Files.Entries(unwrapDir(d)) }
func (f *scenarioFiles) Stat(d sys.Dir, n string) (sys.FileInfo, error) {
	return f.Files.Stat(unwrapDir(d), n)
}
func (f *scenarioFiles) OpenRegular(d sys.Dir, n string) (sys.RegularFile, error) {
	return f.Files.OpenRegular(unwrapDir(d), n)
}
func (f *scenarioFiles) Lock(ctx context.Context, d sys.Dir, n string) (io.Closer, error) {
	if f.mode == "barrier" {
		f.mode = "" // only the first lock in this child
		ready := os.NewFile(3, "ready")
		gate := os.NewFile(4, "gate")
		if _, err := ready.Write([]byte{1}); err != nil {
			return nil, err
		}
		ready.Close()
		if _, err := io.Copy(io.Discard, gate); err != nil {
			return nil, err
		}
		gate.Close()
	}
	return f.Files.Lock(ctx, unwrapDir(d), n)
}

func (f *scenarioFiles) TryLock(d sys.Dir, n string) (io.Closer, error) {
	return f.Files.TryLock(unwrapDir(d), n)
}

func killedAdd(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) > 0 {
		ts.Fatalf("usage: killed-add")
	}
	ts.Setenv("PBRAIN_TEST_FAULT", "kill")
	err := ts.Exec("pbrain-test", "add", "--title", "Published", "--body", "the entire body", "--author", "person")
	ts.Setenv("PBRAIN_TEST_FAULT", "")
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.Sys().(syscall.WaitStatus).Signal() != syscall.SIGKILL {
		ts.Fatalf("expected SIGKILL at publication, got %v", err)
	}
}

func killedIndex(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 0 {
		ts.Fatalf("usage: killed-index")
	}
	ts.Setenv("PBRAIN_TEST_FAULT", "kill-index")
	err := ts.Exec("pbrain-test", "sync")
	ts.Setenv("PBRAIN_TEST_FAULT", "")
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.Sys().(syscall.WaitStatus).Signal() != syscall.SIGKILL {
		ts.Fatalf("expected SIGKILL before the index manifest was published, got %v", err)
	}
}

func parallelAdd(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) > 1 {
		ts.Fatalf("usage: parallel-add [replaced-id]")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	readyR, readyW, err := os.Pipe()
	ts.Check(err)
	defer readyR.Close()
	defer readyW.Close()
	gateR, gateW, err := os.Pipe()
	ts.Check(err)
	defer gateR.Close()
	defer gateW.Close()
	deadline, _ := ctx.Deadline()
	ts.Check(readyR.SetReadDeadline(deadline))
	exe, err := os.Executable()
	ts.Check(err)
	var cmds [2]*exec.Cmd
	var outs, diags [2]bytes.Buffer
	for i := range cmds {
		cmd := exec.CommandContext(ctx, exe, "add", "--json", "--title", "Parallel", "--author", "person", "--body", "complete")
		cmd.Args[0] = "pbrain-test"
		if len(args) == 1 {
			cmd.Args = append(cmd.Args, "--replaces", args[0])
		}
		cmd.Dir = ts.MkAbs(".")
		for _, key := range []string{"PATH", "HOME", "GIT_CONFIG_NOSYSTEM", "GIT_CONFIG_GLOBAL", "PBRAIN_TEST_NOW", "TZ", "GOMAXPROCS"} {
			cmd.Env = append(cmd.Env, key+"="+ts.Getenv(key))
		}
		cmd.Env = append(cmd.Env, "PBRAIN_TEST_FAULT=barrier")
		cmd.Stdout, cmd.Stderr = &outs[i], &diags[i]
		cmd.ExtraFiles = []*os.File{readyW, gateR}
		ts.Check(cmd.Start())
		cmds[i] = cmd
		defer func() {
			if cmd.ProcessState == nil {
				cmd.Process.Kill()
				cmd.Wait()
			}
		}()
	}
	readyW.Close()
	var arrived [2]byte
	_, err = io.ReadFull(readyR, arrived[:])
	ts.Check(err)
	gateW.Close()
	ids := map[string]bool{}
	failures := 0
	for i, cmd := range cmds {
		err := cmd.Wait()
		if err != nil {
			var diag struct {
				Error struct{ Code, Message string }
			}
			if cmd.ProcessState.ExitCode() != 1 || outs[i].Len() != 0 || json.Unmarshal(diags[i].Bytes(), &diag) != nil || diag.Error.Code != "invalid" || !strings.Contains(diag.Error.Message, "already replaced") {
				ts.Fatalf("writer failed: %v: %s %s", err, &diags[i], &outs[i])
			}
			failures++
			continue
		}
		var row struct{ ID, Path string }
		ts.Check(json.Unmarshal(outs[i].Bytes(), &row))
		if !strings.HasPrefix(row.ID, "20261121-120000-") || ids[row.ID] {
			ts.Fatalf("unexpected id %q", row.ID)
		}
		ids[row.ID] = true
		if _, err := os.Stat(ts.MkAbs(row.Path)); err != nil {
			ts.Fatalf("published path %q: %v", row.Path, err)
		}
		ts.Setenv(fmt.Sprintf("WINNER%d", len(ids)), row.ID)
	}
	if failures != len(args) {
		ts.Fatalf("expected %d failures, got %d", len(args), failures)
	}
}
