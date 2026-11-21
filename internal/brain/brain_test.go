package brain

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pkieltyka/pbrain/internal/audience"
	"github.com/pkieltyka/pbrain/internal/errs"
	"github.com/pkieltyka/pbrain/internal/gitx"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/sys"
)

// world is a temporary machine: a home with its own Git configuration and a project folder.
type world struct {
	t    *testing.T
	root string // resolved temporary folder
	git  gitx.Runner
	vars map[string]string
}

func newWorld(t *testing.T) *world {
	t.Helper()
	g, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not installed")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "home")
	must(t, os.MkdirAll(home, 0o700))
	w := &world{t: t, root: root, vars: map[string]string{}}
	w.git = gitx.NewRunner(g, []string{"HOME=" + home, "XDG_CONFIG_HOME=" + home, "GIT_CONFIG_NOSYSTEM=1", "PATH=" + os.Getenv("PATH")})
	return w
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func (w *world) path(rel string) string { return filepath.Join(w.root, filepath.FromSlash(rel)) }

func (w *world) write(rel, content string) {
	w.t.Helper()
	must(w.t, os.MkdirAll(filepath.Dir(w.path(rel)), 0o700))
	must(w.t, os.WriteFile(w.path(rel), []byte(content), 0o600))
}

func (w *world) gitIn(rel string, args ...string) {
	w.t.Helper()
	res, err := w.git.Run(context.Background(), w.path(rel), args, nil)
	if err != nil || res.Status != 0 {
		w.t.Fatalf("git %v in %s: %v %d %s", args, rel, err, res.Status, res.Stdout)
	}
}

// project makes a Git work tree at rel with a brain at docs/brain and init's ignore rules.
func (w *world) project(rel string) {
	w.t.Helper()
	must(w.t, os.MkdirAll(w.path(rel), 0o700))
	w.gitIn(rel, "init", "-q")
	w.write(rel+"/docs/brain/pbrain.toml", "")
	w.write(rel+"/docs/brain/.gitignore", "personal/\npersonal.toml\n.pbrain/\n")
}

func (w *world) env(dir string) Env {
	return Env{
		Dir:    w.path(dir),
		Getenv: func(k string) string { return w.vars[k] },
		Git:    w.git,
		Files:  sys.Native(),
	}
}

func (w *world) open(dir string, opt OpenOptions) (*Brain, error) {
	b, err := Open(context.Background(), w.env(dir), opt)
	if err == nil {
		w.t.Cleanup(func() { b.Close() })
	}
	return b, err
}

func code(err error) string {
	var e *errs.Error
	if errors.As(err, &e) {
		return e.Code
	}
	if err == nil {
		return ""
	}
	return "other: " + err.Error()
}

func TestDiscoveryAndInitPrecedence(t *testing.T) {
	w := newWorld(t)
	w.project("p")
	must(t, os.MkdirAll(w.path("p/src/deep"), 0o700))
	for _, from := range []string{"p", "p/src/deep", "p/docs/brain"} {
		b, err := w.open(from, OpenOptions{})
		if err != nil || b.Dir != w.path("p/docs/brain") || b.Root != w.path("p") {
			t.Fatalf("from %s: %v %+v", from, err, b)
		}
	}
	// A custom folder is found from inside it, and by the selector from anywhere.
	w.write("p/notes/kb/pbrain.toml", "search = { limit = 7 }\n")
	w.write("p/notes/kb/.gitignore", "personal/\npersonal.toml\n.pbrain/\n")
	b, err := w.open("p/notes/kb", OpenOptions{})
	if err != nil || b.Dir != w.path("p/notes/kb") || b.Config.Search.Limit != 7 {
		t.Fatalf("inside a custom brain: %v", err)
	}
	b, err = w.open("p/src", OpenOptions{Brain: "../notes/kb"})
	if err != nil || b.Dir != w.path("p/notes/kb") {
		t.Fatalf("--brain: %v", err)
	}
	w.vars["PBRAIN_DIR"] = w.path("p/notes/kb")
	b, err = w.open("p/src", OpenOptions{})
	if err != nil || b.Dir != w.path("p/notes/kb") {
		t.Fatalf("PBRAIN_DIR: %v", err)
	}
	b, err = w.open("p/src", OpenOptions{Brain: w.path("p/docs/brain")})
	if err != nil || b.Dir != w.path("p/docs/brain") {
		t.Fatalf("the flag wins over PBRAIN_DIR: %v", err)
	}
	delete(w.vars, "PBRAIN_DIR")
	if _, err := w.open("p", OpenOptions{Brain: "nowhere"}); code(err) != "no_brain" {
		t.Fatalf("a missing selected brain: %v", err)
	}
	must(t, os.MkdirAll(w.path("empty"), 0o700))
	if _, err := w.open("empty", OpenOptions{}); code(err) != "no_brain" {
		t.Fatalf("no brain: %v", err)
	}
	// A malformed marker stops the walk with invalid_config.
	w.write("p/docs/brain/pbrain.toml", "search = [\n")
	if _, err := w.open("p", OpenOptions{}); code(err) != "invalid_config" {
		t.Fatalf("malformed config: %v", err)
	}
	w.write("p/docs/brain/pbrain.toml", "")
	// A marker that is a link is invalid_config.
	must(t, os.Remove(w.path("p/docs/brain/pbrain.toml")))
	must(t, os.Symlink(w.path("p/notes/kb/pbrain.toml"), w.path("p/docs/brain/pbrain.toml")))
	if _, err := w.open("p", OpenOptions{}); code(err) != "invalid_config" {
		t.Fatalf("a linked marker: %v", err)
	}
}

func TestInitAndRepeat(t *testing.T) {
	w := newWorld(t)
	must(t, os.MkdirAll(w.path("p/src"), 0o700))
	w.gitIn("p", "init", "-q")
	w.write("p/docs/brain/notes/kept.md", "not touched")
	w.write("p/docs/brain/.gitignore", "*.swp") // no final newline
	res, err := Init(context.Background(), w.env("p/src"), InitOptions{})
	if err != nil || res.Path != "docs/brain" || res.NeedsSelector || res.Dir != w.path("p/docs/brain") {
		t.Fatalf("init: %+v %v", res, err)
	}
	got, _ := os.ReadFile(w.path("p/docs/brain/.gitignore"))
	if string(got) != "*.swp\npersonal/\npersonal.toml\n.pbrain/\n" {
		t.Fatalf(".gitignore: %q", got)
	}
	if data, _ := os.ReadFile(w.path("p/docs/brain/notes/kept.md")); string(data) != "not touched" {
		t.Fatal("init touched a note")
	}
	for _, p := range []string{"p/docs/brain/personal", "p/docs/brain/.pbrain"} {
		if _, err := os.Stat(w.path(p)); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("init created %s", p)
		}
	}
	before, _ := os.Stat(w.path("p/docs/brain/.gitignore"))
	if _, err := Init(context.Background(), w.env("p"), InitOptions{}); err != nil {
		t.Fatalf("repeat: %v", err)
	}
	after, _ := os.Stat(w.path("p/docs/brain/.gitignore"))
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("a repeat rewrote an unchanged .gitignore")
	}
	// A second brain in another folder of the project is refused.
	if _, err := Init(context.Background(), w.env("p"), InitOptions{Path: "kb"}); code(err) != "invalid" {
		t.Fatalf("a second brain: %v", err)
	}
	// A custom folder in a fresh project needs a selector.
	must(t, os.MkdirAll(w.path("q"), 0o700))
	w.gitIn("q", "init", "-q")
	res, err = Init(context.Background(), w.env("q"), InitOptions{Path: "kb"})
	if err != nil || res.Path != "kb" || !res.NeedsSelector {
		t.Fatalf("custom folder: %+v %v", res, err)
	}
	// Outside Git the project root is the current directory, and only the walk applies.
	must(t, os.MkdirAll(w.path("plain"), 0o700))
	if _, err := Init(context.Background(), w.env("plain"), InitOptions{}); err != nil {
		t.Fatalf("outside Git: %v", err)
	}
}

func TestInitFindsCustomMarkerInGit(t *testing.T) {
	w := newWorld(t)
	must(t, os.MkdirAll(w.path("p"), 0o700))
	w.gitIn("p", "init", "-q")
	w.write("p/kb/pbrain.toml", "")
	w.gitIn("p", "add", "kb/pbrain.toml")
	if _, err := Init(context.Background(), w.env("p"), InitOptions{}); code(err) != "invalid" || !strings.Contains(err.Error(), "kb/pbrain.toml") {
		t.Fatalf("a tracked custom marker: %v", err)
	}
	w.gitIn("p", "rm", "-q", "--cached", "kb/pbrain.toml")
	if _, err := Init(context.Background(), w.env("p"), InitOptions{}); code(err) != "invalid" {
		t.Fatalf("an untracked custom marker: %v", err)
	}
	w.write("p/.gitignore", "kb/\n")
	if _, err := Init(context.Background(), w.env("p"), InitOptions{}); err != nil {
		t.Fatalf("an ignored marker blocked init: %v", err)
	}
	// A separate nested checkout gets its own brain.
	must(t, os.MkdirAll(w.path("p/vendor/other"), 0o700))
	w.gitIn("p/vendor/other", "init", "-q")
	if _, err := Init(context.Background(), w.env("p/vendor/other"), InitOptions{}); err != nil {
		t.Fatalf("a nested checkout: %v", err)
	}
}

func TestInitRefusesLinkedIgnoreFile(t *testing.T) {
	w := newWorld(t)
	must(t, os.MkdirAll(w.path("p/docs/brain"), 0o700))
	w.gitIn("p", "init", "-q")
	w.write("elsewhere", "keep")
	must(t, os.Symlink(w.path("elsewhere"), w.path("p/docs/brain/.gitignore")))
	if _, err := Init(context.Background(), w.env("p"), InitOptions{}); code(err) != "invalid" {
		t.Fatalf("a linked .gitignore: %v", err)
	}
	if data, _ := os.ReadFile(w.path("elsewhere")); string(data) != "keep" {
		t.Fatal("the link's target changed")
	}
}

func TestGuardRealIndexAndRoot(t *testing.T) {
	w := newWorld(t)
	w.project("p")
	if _, err := w.open("p", OpenOptions{}); err != nil {
		t.Fatalf("a clean brain: %v", err)
	}
	// A force-added personal note is caught whatever GIT_INDEX_FILE the caller has.
	w.write("p/docs/brain/personal/notes/a.md", "x")
	w.gitIn("p", "add", "-f", "docs/brain/personal/notes/a.md")
	if _, err := w.open("p", OpenOptions{}); code(err) != "personal_exposed" {
		t.Fatalf("a tracked personal file: %v", err)
	}
	w.gitIn("p", "rm", "-q", "--cached", "docs/brain/personal/notes/a.md")
	if _, err := w.open("p", OpenOptions{}); err != nil {
		t.Fatalf("after untracking: %v", err)
	}
	// Not ignored.
	w.write("p/docs/brain/.gitignore", "personal.toml\n.pbrain/\n")
	if _, err := w.open("p", OpenOptions{}); code(err) != "personal_exposed" {
		t.Fatalf("personal/ not ignored: %v", err)
	}
	w.write("p/docs/brain/.gitignore", "personal/\npersonal.toml\n.pbrain/\n")
	// A tracked file at the protected path itself.
	w.write("p/docs/brain/personal.toml", "")
	w.gitIn("p", "add", "-f", "docs/brain/personal.toml")
	if _, err := w.open("p", OpenOptions{}); code(err) != "personal_exposed" {
		t.Fatalf("a tracked personal.toml: %v", err)
	}
	w.gitIn("p", "rm", "-q", "--cached", "docs/brain/personal.toml")
	// A linked work tree uses its own index.
	w.gitIn("p", "add", "docs/brain/pbrain.toml", "docs/brain/.gitignore")
	w.gitIn("p", "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "brain")
	w.gitIn("p", "worktree", "add", "-q", w.path("wt"))
	if _, err := w.open("wt", OpenOptions{}); err != nil {
		t.Fatalf("a linked work tree: %v", err)
	}
	w.write("wt/docs/brain/personal/x.md", "x")
	w.gitIn("wt", "add", "-f", "docs/brain/personal/x.md")
	if _, err := w.open("wt", OpenOptions{}); code(err) != "personal_exposed" {
		t.Fatalf("a file tracked in the linked tree's index: %v", err)
	}
	if _, err := w.open("p", OpenOptions{}); err != nil {
		t.Fatalf("the main tree saw the linked tree's index: %v", err)
	}
}

func TestGuardMissingPathsAndNestedGit(t *testing.T) {
	w := newWorld(t)
	w.project("p")
	// A .git entry at any depth of a protected folder: a folder, or a gitfile.
	w.write("p/docs/brain/personal/a/b/.git", "gitdir: elsewhere")
	_, err := w.open("p", OpenOptions{})
	if code(err) != "personal_exposed" || strings.Contains(err.Error(), w.root) {
		t.Fatalf("a nested gitfile: %v", err)
	}
	must(t, os.RemoveAll(w.path("p/docs/brain/personal")))
	must(t, os.MkdirAll(w.path("p/docs/brain/.pbrain/.git"), 0o700))
	if _, err := w.open("p", OpenOptions{}); code(err) != "personal_exposed" {
		t.Fatalf("a .git folder at the top of .pbrain: %v", err)
	}
	must(t, os.RemoveAll(w.path("p/docs/brain/.pbrain")))
	// A protected folder that is a link.
	must(t, os.MkdirAll(w.path("target"), 0o700))
	must(t, os.Symlink(w.path("target"), w.path("p/docs/brain/personal")))
	if _, err := w.open("p", OpenOptions{}); code(err) != "personal_exposed" {
		t.Fatalf("a linked personal folder: %v", err)
	}
	must(t, os.Remove(w.path("p/docs/brain/personal")))
	// An external personal section outside every tree needs only the no-.git rule.
	must(t, os.MkdirAll(w.path("ext/notes"), 0o700))
	w.write("p/docs/brain/personal.toml", "path = \""+w.path("ext")+"\"\n")
	b, err := w.open("p", OpenOptions{})
	if err != nil || !b.Personal.Reachable || b.Personal.Dir != w.path("ext") {
		t.Fatalf("an external section: %v", err)
	}
	// Inside another work tree it must be ignored there.
	w.gitIn("ext", "init", "-q")
	if _, err := w.open("p", OpenOptions{}); code(err) != "personal_exposed" {
		t.Fatalf("an external section that is a repository: %v", err)
	}
	must(t, os.RemoveAll(w.path("ext/.git")))
	must(t, os.MkdirAll(w.path("other"), 0o700))
	w.gitIn("other", "init", "-q")
	must(t, os.MkdirAll(w.path("other/private"), 0o700))
	w.write("p/docs/brain/personal.toml", "path = \""+w.path("other/private")+"\"\n")
	if _, err := w.open("p", OpenOptions{}); code(err) != "personal_exposed" {
		t.Fatalf("an external section not ignored in its tree: %v", err)
	}
	w.write("other/.gitignore", "private/\n")
	if _, err := w.open("p", OpenOptions{}); err != nil {
		t.Fatalf("an ignored external section: %v", err)
	}
	// An unreachable section is not checked; the personal view then fails.
	w.write("p/docs/brain/personal.toml", "path = \""+w.path("gone")+"\"\n")
	b, err = w.open("p", OpenOptions{})
	if err != nil || b.Personal.Reachable {
		t.Fatalf("an unreachable section: %v", err)
	}
	if code(b.RequireView(PersonalView)) != "personal_unavailable" || b.RequireView(SharedView) != nil {
		t.Fatal("RequireView")
	}
	if _, err := b.OpenSection(item.Personal); code(err) != "personal_unavailable" {
		t.Fatalf("OpenSection: %v", err)
	}
	// Git required but not runnable.
	w.write("p/docs/brain/personal.toml", "")
	env := w.env("p")
	env.Git = nil
	if _, err := Open(context.Background(), env, OpenOptions{}); code(err) != "git_required" {
		t.Fatalf("no git: %v", err)
	}
}

func TestDefaultPersonalLifecycle(t *testing.T) {
	w := newWorld(t)
	w.project("p")
	b, err := w.open("p", OpenOptions{})
	must(t, err)
	if err := b.RequireView(PersonalView); err != nil {
		t.Fatal(err)
	}
	if _, err := b.OpenSection(item.Personal); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a missing default section: %v", err)
	}
	if _, err := os.Stat(w.path("p/docs/brain/personal")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("a read created the personal folder")
	}
	h, err := b.EnsurePersonal(context.Background())
	must(t, err)
	h2, err := b.OpenSection(item.Personal)
	if err != nil || h2 != h {
		t.Fatal("EnsurePersonal's handle is not the section's handle")
	}
	if st, err := os.Stat(w.path("p/docs/brain/personal")); err != nil || st.Mode().Perm() != 0o700 {
		t.Fatalf("the created folder: %v", err)
	}
	// With another path set, a valid item left in personal/ fails the configuration.
	must(t, os.MkdirAll(w.path("ext"), 0o700))
	w.write("p/docs/brain/personal.toml", "path = \""+w.path("ext")+"\"\n")
	w.write("p/docs/brain/personal/notes/old.md", "---\nkind: note\ntitle: Old\ncreated: 2026-11-01T00:00:00Z\nauthor: human:x\n---\nbody\n")
	if _, err := w.open("p", OpenOptions{}); code(err) != "invalid_config" {
		t.Fatalf("a dormant default item: %v", err)
	}
	w.write("p/docs/brain/personal/notes/old.md", "no front matter")
	if _, err := w.open("p", OpenOptions{}); err != nil {
		t.Fatalf("a non-item file in personal/: %v", err)
	}
	// The default folder stays protected while dormant.
	b, err = w.open("p", OpenOptions{})
	must(t, err)
	if !slices.ContainsFunc(b.Protected(), func(p ProtectedPath) bool { return p.Path == w.path("p/docs/brain/personal") }) {
		t.Fatal("the dormant default folder is not protected")
	}
	// The personal section may not be the brain or hold it.
	w.write("p/docs/brain/personal.toml", "path = \"..\"\n")
	if _, err := w.open("p", OpenOptions{}); code(err) != "invalid_config" {
		t.Fatalf("a personal section holding the brain: %v", err)
	}
}

func TestIgnoreAndDisplayUseLiteralPaths(t *testing.T) {
	w := newWorld(t)
	w.project("p")
	w.write("p/.gitignore", "docs/brain/tmp*/\n")
	b, err := w.open("p", OpenOptions{})
	must(t, err)
	paths := []IgnorePath{
		{Path: w.path("p/docs/brain/personal"), Directory: true},
		{Path: w.path("p/docs/brain/tmp one"), Directory: true},
		{Path: w.path("p/docs/brain/a b.md")},
		{Path: w.path("p/docs/brain/:colon.md")},
		{Path: w.path("p/docs/brain/new\nline.md")},
		{Path: w.path("p/docs/brain/[x].md")},
		{Path: w.path("outside/x.md")},
	}
	got, err := b.SharedIgnored(context.Background(), paths)
	if err != nil || !slices.Equal(got, []bool{true, true, false, false, false, false, false}) {
		t.Fatalf("SharedIgnored = %v %v", got, err)
	}
	if d := b.DisplayPath(item.Shared, w.path("p/docs/brain/decisions/a b.md")); d != "docs/brain/decisions/a b.md" {
		t.Fatalf("shared display %q", d)
	}
	if d := b.DisplayPath(item.Shared, w.path("elsewhere/x.md")); d != "" {
		t.Fatalf("a path outside its section printed as %q", d)
	}
	must(t, os.MkdirAll(w.path("home/notes"), 0o700))
	w.write("p/docs/brain/personal.toml", "path = \""+w.path("home/notes")+"\"\n")
	b, err = w.open("p", OpenOptions{})
	must(t, err)
	if d := b.DisplayPath(item.Personal, w.path("home/notes/decisions/x.md")); d != "personal:decisions/x.md" {
		t.Fatalf("external personal display %q", d)
	}
	// Outside Git, the anchor is the brain and nothing is ignored.
	w.write("plain/docs/brain/pbrain.toml", "")
	b, err = w.open("plain", OpenOptions{})
	must(t, err)
	if b.Root != w.path("plain/docs/brain") || b.DisplayPath(item.Shared, w.path("plain/docs/brain/notes/n.md")) != "notes/n.md" {
		t.Fatalf("outside Git: %q", b.Root)
	}
	if got, err := b.SharedIgnored(context.Background(), paths[:1]); err != nil || got[0] {
		t.Fatal("outside Git a path was ignored")
	}
}

func TestReadProjectSettings(t *testing.T) {
	c, err := decodeProject(nil)
	if err != nil || c.Search != (Limits{5, 3000}) || c.List != (Limits{20, 2000}) || c.Get.MaxChars != 6000 ||
		c.Prime.MaxChars != 2000 || c.HasTags || c.Tags != nil || c.Semantic.Model != "" {
		t.Fatalf("defaults: %+v %v", c, err)
	}
	c, err = decodeProject([]byte("search = { limit = 50, max_chars = 9000 }\nlist = { limit = 200 }\nsemantic = { model = \"bge-small-en-v1.5\" }\n[tags]\n"))
	if err != nil || c.Search != (Limits{50, 9000}) || c.List.Limit != 200 || c.List.MaxChars != 2000 ||
		!c.HasTags || len(c.Tags) != 0 || c.Semantic.Model != "bge-small-en-v1.5" {
		t.Fatalf("set values: %+v %v", c, err)
	}
	c, err = decodeProject([]byte("[tags]\ncache = \"Caching layers\"\nci = \"Pipelines\"\n"))
	if err != nil || c.Tags["cache"] != "Caching layers" || len(c.Tags) != 2 {
		t.Fatalf("tags: %+v %v", c, err)
	}
	for _, bad := range []string{
		"search = { limit = 0 }", "search = { limit = 51 }", "list = { limit = 201 }",
		"get = { max_chars = 0 }", "prime = { max_chars = -1 }", "search = { limit = \"5\" }",
		"[tags]\nCache = \"x\"", "[tags]\ncache = \"\"", "[tags]\ncache = \"two\\nlines\"", "[tags]\ncache = 3",
		"search = [",
	} {
		if _, err := decodeProject([]byte(bad)); code(err) != "invalid_config" {
			t.Errorf("%q: %v", bad, err)
		}
	}
	l, err := decodeLocal([]byte("model_path = \"/opt/models/x\"\nauthor = \"human:p\"\nfuture = 1\n"))
	if err != nil || l.config.ModelPath != "/opt/models/x" || l.config.Author != "human:p" {
		t.Fatalf("local: %+v %v", l, err)
	}
}

func TestInvalidRuleDoesNotStopSearch(t *testing.T) {
	w := newWorld(t)
	w.project("p")
	w.write("p/docs/brain/personal.toml", "[[audience]]\nmatch = \"docs.google.com\"\nto = \"shared\"\n")
	b, err := w.open("p", OpenOptions{})
	if err != nil {
		t.Fatalf("an invalid rule stopped the open: %v", err)
	}
	if _, err := b.AudienceRules(); code(err) != "invalid_config" {
		t.Fatalf("AudienceRules: %v", err)
	}
	if _, err := ReadLocal(b.dir); code(err) != "invalid_config" {
		t.Fatalf("ReadLocal: %v", err)
	}
	if _, _, err := EditAudience(b.dir, func(r []audience.Rule) ([]audience.Rule, error) { return r, nil }); code(err) != "invalid_config" {
		t.Fatalf("EditAudience: %v", err)
	}
	w.write("p/docs/brain/personal.toml", "audience = 3\n")
	if _, err := w.open("p", OpenOptions{}); err != nil {
		t.Fatalf("a wrong-typed audience stopped the open: %v", err)
	}
	for _, bad := range []string{"path = 3\n", "path = [\n"} {
		w.write("p/docs/brain/personal.toml", bad)
		if _, err := w.open("p", OpenOptions{}); code(err) != "invalid_config" {
			t.Fatalf("%q: %v", bad, err)
		}
	}
}

func TestLocalSettingsRoundTrip(t *testing.T) {
	w := newWorld(t)
	w.project("p")
	w.write("p/docs/brain/personal.toml", "# mine\npath = \"../../ext\"\nauthor = \"human:p\"\nmodel_path = \"/m\"\n")
	must(t, os.MkdirAll(w.path("p/ext"), 0o700))
	w.write("p/.gitignore", "ext/\n")
	b, err := w.open("p", OpenOptions{})
	must(t, err)
	if b.Personal.Dir != w.path("p/ext") {
		t.Fatalf("a relative path starts at the brain: %s", b.Personal.Dir)
	}
	data, changed, err := EditAudience(b.dir, func(r []audience.Rule) ([]audience.Rule, error) { return r, nil })
	if err != nil || changed || data != nil {
		t.Fatalf("a no-op edit: %v %v", changed, err)
	}
	data, changed, err = EditAudience(b.dir, func(r []audience.Rule) ([]audience.Rule, error) {
		return audience.Set(r, "https://Example.com/team", audience.Personal)
	})
	if err != nil || !changed {
		t.Fatalf("an edit: %v", err)
	}
	l, err := decodeLocal(data)
	if err != nil || l.config.Path != "../../ext" || l.config.Author != "human:p" || l.config.ModelPath != "/m" ||
		len(l.config.Audience) != 1 || l.config.Audience[0].Match != "example.com/team" {
		t.Fatalf("the rewrite lost a value: %+v %v\n%s", l.config, err, data)
	}
	if _, _, err := EditAudience(b.dir, func([]audience.Rule) ([]audience.Rule, error) { return nil, errors.New("callback") }); err == nil {
		t.Fatal("a callback error was swallowed")
	}
}

func TestAuthorPrecedenceAndWorktree(t *testing.T) {
	w := newWorld(t)
	w.project("p")
	w.gitIn("p", "config", "user.name", "Brain Repo")
	must(t, os.MkdirAll(w.path("caller"), 0o700))
	w.gitIn("caller", "init", "-q")
	w.gitIn("caller", "config", "user.name", "Caller Repo")
	b, err := w.open("caller", OpenOptions{Brain: w.path("p/docs/brain")})
	must(t, err)
	ctx := context.Background()
	if a, err := b.ResolveAuthor(ctx, ""); err != nil || a != "human:Brain Repo" {
		t.Fatalf("git: %q %v", a, err)
	}
	b.Local.Author = "human:local"
	if a, _ := b.ResolveAuthor(ctx, ""); a != "human:local" {
		t.Fatalf("local: %q", a)
	}
	w.vars["PBRAIN_AUTHOR"] = "agent:env"
	if a, _ := b.ResolveAuthor(ctx, ""); a != "agent:env" {
		t.Fatalf("env: %q", a)
	}
	if a, _ := b.ResolveAuthor(ctx, "agent:flag"); a != "agent:flag" {
		t.Fatalf("flag: %q", a)
	}
	if _, err := b.ResolveAuthor(ctx, "two\nlines"); code(err) != "invalid" {
		t.Fatalf("a bad flag: %v", err)
	}
	w.vars["PBRAIN_AUTHOR"] = strings.Repeat("x", 201)
	if _, err := b.ResolveAuthor(ctx, ""); code(err) != "invalid" {
		t.Fatalf("a bad higher choice was skipped: %v", err)
	}
}

func TestBorrowedRoots(t *testing.T) {
	w := newWorld(t)
	w.project("p")
	w.write("p/docs/brain/personal/notes/x.md", "x")
	b, err := Open(context.Background(), w.env("p"), OpenOptions{})
	must(t, err)
	s1, _ := b.OpenSection(item.Shared)
	s2, _ := b.OpenSection(item.Shared)
	p1, _ := b.OpenSection(item.Personal)
	p2, _ := b.OpenSection(item.Personal)
	if s1 != s2 || p1 != p2 || p1 == nil {
		t.Fatal("repeated opens do not share a handle")
	}
	if _, err := s1.Sub(".pbrain", false); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("absent .pbrain: %v", err)
	}
	if _, err := os.Stat(w.path("p/docs/brain/.pbrain")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("a create=false open made .pbrain")
	}
	must(t, b.Close())
	must(t, b.Close())
	if _, err := b.OpenSection(item.Shared); err == nil {
		t.Fatal("a closed brain still opens sections")
	}
}
