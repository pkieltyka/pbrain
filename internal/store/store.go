// Package store: exported surface from the pbrain specs; bodies are filled in by later phases.
package store

import (
	"context"

	"github.com/pkieltyka/pbrain/internal/brain"
	"github.com/pkieltyka/pbrain/internal/item"
)

type BodyPolicy uint8 // KeepAll, KeepNone
type Skipped struct {
	Section      item.Section
	Path         string // absolute, for caller's display-path policy
	Code         string // bad_front_matter or not_an_item
	Reason       string // fixed category, never file content
	Line, Column int    // 1-based when available, otherwise zero
}
type Catalog struct { /* immutable after Scan */
}
type WritePlan struct {
	Dest     item.Section
	Lookup   brain.View
	Disclose brain.View
}
type Tx struct { /* owns section lock and child handles */
}
type Commit struct{ Cleanup []Cleanup }
type Cleanup uint8 // TempLeft, FolderSyncFailed

var ErrChanged error
var ErrNameExists error

func Scan(ctx context.Context, b *brain.Brain, v brain.View, keep BodyPolicy) (*Catalog, error) {
	panic("not implemented")
}
func (c *Catalog) Items() []*item.Item                { panic("not implemented") }
func (c *Catalog) Lookup(id string) []*item.Item      { panic("not implemented") }
func (c *Catalog) Skipped() []Skipped                 { panic("not implemented") }
func (c *Catalog) Body(it *item.Item) ([]byte, error) { panic("not implemented") }
func OpenInput(b *brain.Brain, dest item.Section, path string) ([]byte, error) {
	panic("not implemented")
}

func Begin(ctx context.Context, b *brain.Brain, section item.Section) (*Tx, error) {
	panic("not implemented")
}
func (tx *Tx) Close() error { panic("not implemented") }
func (tx *Tx) CheckDestination(ctx context.Context, kind item.Kind, id string) error {
	panic("not implemented")
}
func (tx *Tx) Publish(ctx context.Context, kind item.Kind, id string, data []byte) (commit Commit, err error) {
	panic("not implemented")
}
func (tx *Tx) ReadFile(it *item.Item) (data []byte, version item.Version, err error) {
	panic("not implemented")
}
func (tx *Tx) ReplaceItem(it *item.Item, want item.Version, data []byte) (commit Commit, err error) {
	panic("not implemented")
}
func (tx *Tx) RemoveItem(it *item.Item) (commit Commit, err error)      { panic("not implemented") }
func (tx *Tx) PutOriginal(digest [32]byte, data []byte) error           { panic("not implemented") }
func (tx *Tx) RemoveOriginal(digest [32]byte) (removed bool, err error) { panic("not implemented") }
func (tx *Tx) ReplaceSettings(data []byte) (commit Commit, err error)   { panic("not implemented") }
