package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pkieltyka/pbrain/internal/brain"
	"github.com/pkieltyka/pbrain/internal/graph"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/query"
	"github.com/pkieltyka/pbrain/internal/store"
	"github.com/pkieltyka/pbrain/internal/sys"
)

const (
	calibration = "calibration"
	heldOut     = "held-out"
	answerable  = "answerable"
	unanswered  = "unanswerable"
	longSource  = "long-document"
)

var frozenTime = time.Date(2026, 11, 21, 18, 0, 0, 0, time.UTC)

type question struct {
	ID         string   `json:"id"`
	Text       string   `json:"q"`
	Kind       string   `json:"kind"`
	Style      string   `json:"style"`
	Expect     []string `json:"expect"`
	Half       string   `json:"half"`
	Regression bool     `json:"regression"`
	Evidence   string   `json:"evidence"`
}

type fixture struct {
	root, dir string
	overlay   string
	questions []question
}

func loadFixture(ctx context.Context, source string) (*fixture, error) {
	f, err := os.Open(filepath.Join(source, "questions.json"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	questions, err := readQuestions(f)
	if err != nil {
		return nil, err
	}
	scratch, err := os.MkdirTemp("", "pbrain-calibrate-")
	if err != nil {
		return nil, err
	}
	root, err := filepath.EvalSymlinks(scratch)
	if err != nil {
		os.RemoveAll(scratch)
		return nil, err
	}
	fix := &fixture{root: root, dir: filepath.Join(root, "brain"), overlay: filepath.Join(source, "personal-overlay"), questions: questions}
	if err := fix.prepare(ctx, source); err != nil {
		fix.close()
		return nil, err
	}
	return fix, nil
}

func (f *fixture) close() { os.RemoveAll(f.root) }

func (f *fixture) prepare(ctx context.Context, source string) error {
	for dir := f.root; ; dir = filepath.Dir(dir) {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("temporary directory must be outside a Git worktree: %s", dir)
		}
		if dir == filepath.Dir(dir) {
			break
		}
	}
	if err := copyFixtureTree(f.dir, filepath.Join(source, "brain")); err != nil {
		return err
	}
	// Calibration uses the published default budgets and a separate personal overlay.
	for _, name := range []string{"personal", "personal.toml"} {
		if _, err := os.Lstat(filepath.Join(f.dir, name)); !errors.Is(err, os.ErrNotExist) {
			return errors.New("fixture brain must keep personal items in personal-overlay")
		}
	}
	marker := filepath.Join(f.dir, "pbrain.toml")
	if err := os.Remove(marker); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	config, err := os.OpenFile(marker, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if err := config.Close(); err != nil {
		return err
	}
	c, err := scan(ctx, f.dir, brain.SharedView)
	if err != nil {
		return err
	}
	return validateQuestions(f.questions, c)
}

func copyFixtureTree(dest, source string) error {
	root, err := os.OpenRoot(source)
	if err != nil {
		return err
	}
	defer root.Close()
	tree := root.FS()
	if err := fs.WalkDir(tree, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && !entry.Type().IsRegular() {
			return fmt.Errorf("fixture file %q is not regular", path)
		}
		return nil
	}); err != nil {
		return err
	}
	return os.CopyFS(dest, tree)
}

func readQuestions(r io.Reader) ([]question, error) {
	d := json.NewDecoder(r)
	d.DisallowUnknownFields()
	var qs []question
	if err := d.Decode(&qs); err != nil {
		return nil, fmt.Errorf("questions.json: %w", err)
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, errors.New("questions.json: trailing data")
	}
	return qs, nil
}

type corpus struct {
	cat   *store.Catalog
	graph *graph.Graph
}

func scan(ctx context.Context, dir string, view brain.View) (corpus, error) {
	b, err := brain.Open(ctx, brain.Env{Dir: dir, Files: sys.Native()}, brain.OpenOptions{})
	if err != nil {
		return corpus{}, err
	}
	defer b.Close()
	cat, err := store.Scan(ctx, b, view, store.KeepAll)
	if err != nil {
		return corpus{}, err
	}
	if skipped := cat.Skipped(); len(skipped) != 0 {
		return corpus{}, fmt.Errorf("fixture has %d skipped files, first: %s", len(skipped), skipped[0].Path)
	}
	return corpus{cat: cat, graph: graph.Build(cat.Items())}, nil
}

func validateQuestions(qs []question, c corpus) error {
	ids := make(map[string]bool, len(qs))
	halves := map[string]int{calibration: 0, heldOut: 0}
	answers := 0
	for _, q := range qs {
		if q.ID == "" || ids[q.ID] {
			return fmt.Errorf("absent or repeated question id %q", q.ID)
		}
		ids[q.ID] = true
		if _, err := query.Parse(q.Text); err != nil {
			return fmt.Errorf("%s: %w", q.ID, err)
		}
		switch q.Kind {
		case answerable:
			answers++
			switch q.Style {
			case "reworded", "short", "identifier", "superseded", longSource:
			default:
				return fmt.Errorf("%s: unsupported answerable style %q", q.ID, q.Style)
			}
			if q.Half != "" || q.Regression || len(q.Expect) == 0 {
				return fmt.Errorf("%s: answerable question needs expected ids and no negative label", q.ID)
			}
			for _, id := range q.Expect {
				items := c.cat.Lookup(id)
				if len(items) != 1 || items[0].ID != id || !c.graph.Current(items[0]) {
					return fmt.Errorf("%s: expected id %q must identify a current shared item", q.ID, id)
				}
			}
			if err := validateEvidence(q, c.cat); err != nil {
				return fmt.Errorf("%s: %w", q.ID, err)
			}
		case unanswered:
			if q.Style != "far" && q.Style != "near" && q.Style != "personal-only" {
				return fmt.Errorf("%s: unsupported unanswerable style %q", q.ID, q.Style)
			}
			if _, ok := halves[q.Half]; !ok || len(q.Expect) != 0 || q.Evidence != "" {
				return fmt.Errorf("%s: unanswerable question needs a recorded half and no expected answer", q.ID)
			}
			halves[q.Half]++
		default:
			return fmt.Errorf("%s: unknown question kind %q", q.ID, q.Kind)
		}
	}
	if answers < 30 || halves[calibration] < 30 || halves[heldOut] < 30 {
		return fmt.Errorf("fixture needs 30 answerable and 30 unanswerable per half (got %d, %d, %d)", answers, halves[calibration], halves[heldOut])
	}
	return nil
}

func validateEvidence(q question, cat *store.Catalog) error {
	if q.Style != longSource {
		if q.Evidence != "" {
			return errors.New("evidence belongs to long-document questions")
		}
		return nil
	}
	want := normalize(q.Evidence)
	if len(q.Expect) != 1 || want == "" || utf8.RuneCountInString(q.Evidence) > 120 {
		return errors.New("long-document question needs one id and a quotation of at most 120 code points")
	}
	body, err := cat.Body(cat.Lookup(q.Expect[0])[0])
	if err != nil {
		return err
	}
	text := normalize(string(body))
	i := strings.Index(text, want)
	if i < 0 || strings.Contains(text[i+1:], want) {
		return errors.New("quotation must occur exactly once in the expected body")
	}
	return nil
}

func normalize(text string) string { return strings.Join(strings.Fields(text), " ") }

func (f *fixture) installPersonal(ctx context.Context) ([]string, error) {
	if err := copyFixtureTree(filepath.Join(f.dir, "personal"), f.overlay); err != nil {
		return nil, err
	}
	c, err := scan(ctx, f.dir, brain.PersonalView)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, it := range c.cat.Items() {
		if it.Section == item.Personal {
			ids = append(ids, it.ID)
		}
	}
	if len(ids) == 0 {
		return nil, errors.New("fixture personal overlay contains no items")
	}
	return ids, nil
}
