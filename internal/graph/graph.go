// Package graph: exported surface from the pbrain specs; bodies are filled in by later phases.
package graph

import "github.com/pkieltyka/pbrain/internal/item"

type Graph struct { /* unexported, immutable after Build */
}

func Build(items []*item.Item) *Graph { panic("not implemented") }

func (g *Graph) Current(it *item.Item) bool                   { panic("not implemented") }
func (g *Graph) Replacement(it *item.Item) []*item.Item       { panic("not implemented") }
func (g *Graph) CurrentSuccessors(it *item.Item) []*item.Item { panic("not implemented") }
func (g *Graph) History(it *item.Item) []*item.Item           { panic("not implemented") }
func (g *Graph) RelatesTo(it *item.Item) []*item.Item         { panic("not implemented") }
func (g *Graph) NewerRelated(it *item.Item) []*item.Item      { panic("not implemented") }
func (g *Graph) Unresolved(it *item.Item) int                 { panic("not implemented") }
func (g *Graph) Findings() []LinkFinding                      { panic("not implemented") }

type Impact struct {
	CurrentAgain []*item.Item
	Dangling     []*item.Item
}

func Effect(items []*item.Item, target *item.Item) Impact { panic("not implemented") }

// Replaces: the items this one replaces through counted links (valid replaces links,
// and accepted stamps that name it), in history order. get --history prints them.
func (g *Graph) Replaces(it *item.Item) []*item.Item { panic("not implemented") }

// Shown returns the ids an item's own replaces and relates fields name that exist in the
// command's view, in field order and deduplicated by folded id, each as written in the file.
// get prints these on its replaces: and relates: lines; everything else is counted by Unresolved.
func (g *Graph) Shown(it *item.Item, f Field) []string { panic("not implemented") }

// CheckNewReplaces applies spec 4.2 to links a write is about to create, against this graph.
// For each named id: it must resolve (lookup rules of section 3.1, in the new item's section and view),
// pass the standing and kind rules for the new item's header, and name a current item.
// It cannot form a cycle, given one precondition the caller guarantees: no existing item
// names the new id, in replaces or in a legacy stamp. writes reserves those ids when it draws
// the id (NewID's used callback also returns true for every folded id named by any item's
// replaces field or stamp in the section, dangling ones included), so the new item has no
// incoming link when it is published.
// Errors (*errs.Error):
//
//	not_found     the id resolves to nothing in the view (the write's Lookup view)
//	ambiguous_id  more than one item in the view has it
//	invalid       another section, or the standing or kind rule fails
//	              (message: "replaces <id>: <rule>")
//	invalid       the target is already replaced (spec 4.2); the returned slice is then that
//	              target's current successors, for writes to name if Disclose allows
//
// On success the returned slice is the resolved targets, in the order of ids.
func (g *Graph) CheckNewReplaces(newSection item.Section, newHeader *item.Header, ids []string) ([]*item.Item, error) {
	panic("not implemented")
}

type Field uint8 // FieldReplaces, FieldRelates, FieldStatus (the legacy stamp)

type LinkFinding struct {
	Code       string     // replace_invalid | replace_cycle | replace_fork | legacy_stamp_ignored | dangling_link | personal_reference
	Item       *item.Item // the item whose file holds the link (the replaced item, for replace_fork)
	Field      Field
	Target     string       // the id as written; empty for dangling_link and personal_reference, which name no target
	Reason     Reason       // why, for replace_invalid and legacy_stamp_ignored
	Successors []*item.Item // replace_fork: the current successors
}
type Reason uint8 // Ambiguous, OtherSection, Standing, DecisionKind, SourceKind, AlreadyLinked, Missing, InCycle
