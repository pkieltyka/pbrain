// Package brain: exported surface from the pbrain specs; bodies are filled in by later phases.
package brain

import (
	"context"
	"time"

	"github.com/pkieltyka/pbrain/internal/audience"
	"github.com/pkieltyka/pbrain/internal/gitx"
	"github.com/pkieltyka/pbrain/internal/item"
	"github.com/pkieltyka/pbrain/internal/sys"
)

type Env struct {
	Dir    string // absolute command working directory
	Getenv func(string) string
	Git    gitx.Runner
	Now    func() time.Time
	Files  sys.Files
}

type OpenOptions struct{ Brain string } // raw --brain value, empty when absent
type InitOptions struct {
	OpenOptions
	Path string // raw --path, empty when absent
}
type InitResult struct {
	Dir           string // canonical target, internal only
	Path          string // display path under project root, or "." at its root
	NeedsSelector bool
}
type ProtectedPath struct {
	Path      string // absolute, canonical when reachable
	Directory bool   // intended role, also for a missing path
}
type IgnorePath struct {
	Path      string // absolute and inside the brain's worktree, if any
	Directory bool
}

func Open(ctx context.Context, env Env, opt OpenOptions) (*Brain, error) { panic("not implemented") }
func Init(ctx context.Context, env Env, opt InitOptions) (InitResult, error) {
	panic("not implemented")
}
func (b *Brain) Files() sys.Files                                    { panic("not implemented") }
func (b *Brain) CallerDir() string                                   { panic("not implemented") }
func (b *Brain) OpenSection(section item.Section) (sys.Dir, error)   { panic("not implemented") }
func (b *Brain) EnsurePersonal(ctx context.Context) (sys.Dir, error) { panic("not implemented") }
func (b *Brain) Close() error                                        { panic("not implemented") }
func (b *Brain) RequireView(view View) error                         { panic("not implemented") }
func (b *Brain) Protected() []ProtectedPath                          { panic("not implemented") }
func (b *Brain) SharedIgnored(ctx context.Context, paths []IgnorePath) ([]bool, error) {
	panic("not implemented")
}
func (b *Brain) DisplayPath(section item.Section, path string) string { panic("not implemented") }
func (b *Brain) ResolveAuthor(ctx context.Context, explicit string) (string, error) {
	panic("not implemented")
}
func (b *Brain) AudienceRules() ([]audience.Rule, error) { panic("not implemented") }
func ReadLocal(dir sys.Dir) (LocalConfig, error)         { panic("not implemented") }
func EditAudience(dir sys.Dir, edit func([]audience.Rule) ([]audience.Rule, error)) (data []byte, changed bool, err error) {
	panic("not implemented")
}

type View uint8 // SharedView, PersonalView (shared and personal together)

type Brain struct {
	Dir      string      // the brain folder, absolute, resolved
	Root     string      // the path anchor (section 4.5)
	Config   Config      // pbrain.toml
	Local    LocalConfig // personal.toml; zero value if absent
	Shared   SectionRoot
	Personal SectionRoot // Reachable is false when its folder cannot be reached
}

type SectionRoot struct {
	Section   item.Section
	Dir       string // items live under it
	DataDir   string // Dir + "/.pbrain"
	Reachable bool
}

type Config struct {
	Search, List Limits // Limit, MaxChars
	Get, Prime   struct{ MaxChars int }
	Tags         map[string]string // declared tags; nil when there is no [tags] table
	HasTags      bool
	Semantic     struct{ Model string } // "" when semantic search is off; the name is checked by embed, not here
}

type LocalConfig struct {
	Path      string
	Author    string
	Audience  []audience.Rule
	ModelPath string // a copy of the model on this machine; "" when unset
}

// Limits are a command's default page size and output budget (pbrain.toml).
type Limits struct{ Limit, MaxChars int }
