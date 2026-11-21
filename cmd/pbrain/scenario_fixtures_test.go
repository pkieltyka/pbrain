package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkieltyka/pbrain/internal/embed/embedtest"
	"github.com/rogpeppe/go-internal/testscript"
)

func scenarioModelKit(path string) (*embedtest.Kit, error) {
	if path == "" {
		return embedtest.New(nil), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var table map[string][]float32
	if err := json.Unmarshal(data, &table); err != nil {
		return nil, err
	}
	return embedtest.New(table), nil
}

// testmodel selects a local model for either the default embedtest keyword
// table or a table supplied by this script. The child still verifies the pins.
func scenarioTestModel(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) < 1 || len(args) > 2 {
		ts.Fatalf("usage: testmodel NAME [KEYWORD-TABLE]")
	}
	var table string
	if len(args) == 2 {
		table = ts.MkAbs(args[1])
	}
	ts.Setenv("PBRAIN_TEST_EMBED_TABLE", table)
	kit, err := scenarioModelKit(table)
	ts.Check(err)
	m, ok := kit.Models[args[0]]
	if !ok {
		ts.Fatalf("unknown test model %q", args[0])
	}
	model := filepath.Join(ts.Getenv("WORK"), args[0])
	ts.Check(os.MkdirAll(model, 0700))
	for _, f := range m.Files {
		ts.Check(os.WriteFile(filepath.Join(model, f.Name), kit.Bytes[f.URL], 0600))
	}
	if len(m.Source) > 0 {
		source := model + "-source"
		ts.Check(os.MkdirAll(source, 0700))
		for _, f := range m.Source {
			ts.Check(os.WriteFile(filepath.Join(source, f.Name), kit.Bytes[f.URL], 0600))
		}
	}
	ts.Check(os.WriteFile(ts.MkAbs("docs/brain/pbrain.toml"), []byte("semantic.model = "+strconv.Quote(m.Name)+"\n"), 0600))
	ts.Check(os.WriteFile(ts.MkAbs("docs/brain/personal.toml"), []byte("model_path = "+strconv.Quote(model)+"\n"), 0600))
}

// damage-index publishes a digest-consistent generation containing a passage
// with a malformed vector. Readers reach the database and its scoring query.
func scenarioDamageIndex(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 1 {
		ts.Fatalf("usage: damage-index INDEX-DIR")
	}
	dir := ts.MkAbs(args[0])
	manifest := filepath.Join(dir, "manifest")
	var fields map[string]json.RawMessage
	ts.Check(json.Unmarshal([]byte(ts.ReadFile(manifest)), &fields))
	var name string
	ts.Check(json.Unmarshal(fields["db"], &name))
	if filepath.Base(name) != name || !strings.HasSuffix(name, ".db") {
		ts.Fatalf("invalid generation name %q", name)
	}
	path := filepath.Join(dir, name)
	db, err := sql.Open("sqlite", path)
	ts.Check(err)
	defer db.Close()
	db.SetMaxOpenConns(1)
	_, err = db.Exec("PRAGMA journal_mode = MEMORY")
	ts.Check(err)
	_, err = db.Exec("PRAGMA ignore_check_constraints = ON")
	ts.Check(err)
	result, err := db.Exec("UPDATE passage SET vector=x'00' WHERE id=(SELECT min(id) FROM passage)")
	ts.Check(err)
	n, err := result.RowsAffected()
	ts.Check(err)
	if n != 1 {
		ts.Fatalf("damaged %d passages, want one", n)
	}
	ts.Check(db.Close())
	data, err := os.ReadFile(path)
	ts.Check(err)
	digest := sha256.Sum256(data)
	fields["sha256"] = json.RawMessage(strconv.Quote(hex.EncodeToString(digest[:])))
	fields["size"] = json.RawMessage(strconv.Itoa(len(data)))
	encoded, err := json.Marshal(fields)
	ts.Check(err)
	ts.Check(os.WriteFile(manifest, encoded, 0600))
}
