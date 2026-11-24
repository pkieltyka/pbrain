package buildcheck

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAikitModuleRemoved(t *testing.T) {
	const aikit = "github.com/pkieltyka/aikit"
	dir := root(t)
	for _, name := range []string{"go.mod", "go.sum"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(data, []byte(aikit)) {
			t.Fatalf("%s still mentions the retired module %s", name, aikit)
		}
	}
	cmd := exec.CommandContext(t.Context(), "go", "list", "-m", "-json", "all")
	cmd.Dir = dir
	cmd.Env = append(cmd.Environ(), "GOWORK=off", "GOFLAGS=")
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			t.Fatalf("go list -m: %v\n%s", err, exit.Stderr)
		}
		t.Fatal(err)
	}
	type module struct {
		Path    string
		Replace *module
	}
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var entry module
		if err := dec.Decode(&entry); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		if entry.Path == aikit || entry.Replace != nil && entry.Replace.Path == aikit {
			t.Fatalf("module graph still includes retired module %s", aikit)
		}
	}
}
