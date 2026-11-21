---
status: complete
---

# Component: graph

Package `internal/graph`. Designed against the functional spec at sha256 bd0ffc7c (sections 2.3, 4.1 to 4.5, 5.2, 7.1 related lines, 7.2, 8.6, 9.1 and scenarios 2, 4, 10 to 13, 21 and 29; sections 4 and 9.1 are unchanged from 4622a82f), architecture sha256 8d40545f (sections 4.3, 7.2, 7.3), and `components/item.md`.

## 1. Purpose and scope

`graph` turns the items of one command's view into the links that count, and answers every question that depends on them:

- which `replaces` links are valid;
- which legacy stamps are accepted;
- which links form cycles;
- which items are current;
- direct replacements and current successors;
- lineage (history) in spec 7.2's order;
- resolved `relates` in both directions;
- unresolved link counts;
- the link findings of `check`;
- the effect of removing one item (`Effect`).

Out of scope:

- **Reading files.** `graph` is pure: it receives `[]*item.Item` and touches no file system, clock or Git.
- **Deciding the view.** The caller passes the items of the command's view. Personal items are absent from the shared view, so `graph` cannot name them there.
- **Cutting output to what a write may disclose** (architecture 10.1). `Effect` returns everything it found in the view it was given; `writes` cuts it to `Disclose`.
- **The three-line cap, the "each item once" rule and the order of related lines in output.** Those are `render`'s (`render.md`). `graph` returns full lists in a defined order.
- **Validating new links before a write.** `writes` calls `CheckNewReplaces` (section 2), which applies the same rules to a link not yet on disk.
- **`duplicate_id` findings.** Those come from the catalog's `Lookup` in `check`. `graph` uses ambiguity only to invalidate links.

Imports: `item` and the standard library.

## 2. Public interface

Unchanged from architecture 4.3 and 7.3:

```go
package graph

type Graph struct{ /* unexported, immutable after Build */ }

func Build(items []*item.Item) *Graph

func (g *Graph) Current(it *item.Item) bool
func (g *Graph) Replacement(it *item.Item) []*item.Item
func (g *Graph) CurrentSuccessors(it *item.Item) []*item.Item
func (g *Graph) History(it *item.Item) []*item.Item
func (g *Graph) RelatesTo(it *item.Item) []*item.Item
func (g *Graph) NewerRelated(it *item.Item) []*item.Item
func (g *Graph) Unresolved(it *item.Item) int
func (g *Graph) Findings() []LinkFinding

type Impact struct {
    CurrentAgain []*item.Item
    Dangling     []*item.Item
}
func Effect(items []*item.Item, target *item.Item) Impact
```

Additions, all inside `graph`. No fixed signature changes.

```go
// Replaces: the items this one replaces through counted links (valid replaces links,
// and accepted stamps that name it), in history order. get --history prints them.
func (g *Graph) Replaces(it *item.Item) []*item.Item

// Shown returns the ids an item's own replaces and relates fields name that exist in the
// command's view, in field order and deduplicated by folded id, each as written in the file.
// get prints these on its replaces: and relates: lines; everything else is counted by Unresolved.
func (g *Graph) Shown(it *item.Item, f Field) []string

// CheckNewReplaces applies spec 4.2 to links a write is about to create, against this graph.
// For each named id: it must resolve (lookup rules of section 3.1, in the new item's section and view),
// pass the standing and kind rules for the new item's header, and name a current item.
// It cannot form a cycle, given one precondition the caller guarantees: no existing item
// names the new id, in replaces or in a legacy stamp. writes reserves those ids when it draws
// the id (NewID's used callback also returns true for every folded id named by any item's
// replaces field or stamp in the section, dangling ones included), so the new item has no
// incoming link when it is published.
// Errors (*errs.Error):
//   not_found     the id resolves to nothing in the view (the write's Lookup view)
//   ambiguous_id  more than one item in the view has it
//   invalid       another section, or the standing or kind rule fails
//                 (message: "replaces <id>: <rule>")
//   invalid       the target is already replaced (spec 4.2); the returned slice is then that
//                 target's current successors, for writes to name if Disclose allows
// On success the returned slice is the resolved targets, in the order of ids.
func (g *Graph) CheckNewReplaces(newSection item.Section, newHeader *item.Header, ids []string) ([]*item.Item, error)

type Field uint8 // FieldReplaces, FieldRelates, FieldStatus (the legacy stamp)

type LinkFinding struct {
    Code   string     // replace_invalid | replace_cycle | replace_fork | legacy_stamp_ignored | dangling_link | personal_reference
    Item   *item.Item // the item whose file holds the link (the replaced item, for replace_fork)
    Field  Field
    Target string     // the id as written; empty for dangling_link and personal_reference, which name no target
    Reason Reason     // why, for replace_invalid and legacy_stamp_ignored
    Successors []*item.Item // replace_fork: the current successors
}
type Reason uint8 // Ambiguous, OtherSection, Standing, DecisionKind, SourceKind, AlreadyLinked, Missing, InCycle
```

`CheckNewReplaces` is how `writes` gets the same rules as the reader. `writes` owns the messages, cut to `Disclose`; this function only classifies. Where `writes` may not name a candidate (a rule-routed personal write, architecture 10.1), it leaves the successors out of the message.

The `relates` ids of a write need only exist in the view (spec 8.2). `writes` checks them through `Catalog.Lookup`, not here.

## 3. Internal design

### 3.1 Build: indexing and lookup

`Build` numbers the items `0..n-1` in the order given; the catalog sorts by section, then path. It keeps:

- `node map[*item.Item]int`;
- `byKey map[string][]int`, from `item.Key` (the folded id) to every item with it, in input order.

**Resolution view of a holder.** A link is resolved in the view of the item that holds it (spec 5.2, architecture 7.2):

- the holder is shared: candidates are the shared items with that key;
- the holder is personal: candidates are all items with that key, both sections. Personal items exist in the input only when the command's view is personal.

**`resolve(holder, id)`** returns one of:

- `One(j)`: exactly one candidate;
- `Many`: more than one candidate;
- `None`: no candidate.

For `replaces`, `One(j)` in the other section is `OtherSection`, and `Many` is `Ambiguous`. Both make the link invalid. Legacy stamps do not use `resolve`: they are looked up in their own section only (step 4). A personal holder whose id names one shared and one personal item gets `Many`, as spec 5.2 says of an id present in both sections.

### 3.2 Build: steps

The order of spec 4.2 and 4.3, as in architecture 7.2.

**Step 1: candidate `replaces` links.** For every item N, in input order, and each distinct folded id in `N.Header.Replaces`, in field order, `resolve(N, id)`:

- `None`: no link.
  - If N is shared and the input holds a personal item with that key, the finding is `personal_reference` (error). That only happens in the personal view.
  - Otherwise the finding is `dangling_link` (warning, `Target` empty).
  - Never `replace_invalid` (spec 4.2).
- `Many` or `OtherSection`: `replace_invalid` (error) with that reason; no link.
- `One(O)` with `O == N`: a self-link, so `replace_cycle` (spec 4.2 "an item does not replace itself"); no link.
- `One(O)`: go to step 2.

**Step 2: standing and kind rules.** For a candidate link N → O, with `s = Header.Standing`:

- `s(N) < s(O)`: `Standing`.
- O is a decision and N is not: `DecisionKind`.
- Exactly one of N and O is a source: `SourceKind`.

The first rule that fails, in that order, gives `replace_invalid` with its reason. A link that passes is **valid-before-cycles**.

**Step 3: cycles.** Run Tarjan's strongly-connected-components algorithm, iterative (no recursion, so a 5,000-item chain cannot overflow the stack), over the valid-before-cycles links. Every link whose two ends lie in one component of more than one item is dropped, with `replace_cycle` (error) on the holder. The remaining links are the **valid replaces links**, and they form an acyclic graph.

**Step 4: legacy stamps.** For every item S with `S.Header.LegacyStamp()` ok, the candidate link is F → S, where F is the named item. The stem is looked up **among the items of S's own section only**: "the id of exactly one item in the same section" (spec 4.3). Unlike `replaces`, an item of the other section with the same stem neither counts nor makes the stem ambiguous: a personal stamp with exactly one personal target keeps it in the personal view, even when a shared note has the same stem. Then, in order:

- No item of S's section has the stem (`Missing`), or more than one has (`Ambiguous`): ignored, with `legacy_stamp_ignored` (warning).
- A valid `replaces` link already points at S: ignored, `AlreadyLinked`. The new form wins (spec 4.3).
- Otherwise a candidate stamp.

No standing check applies (spec 4.3). Then judge the stamps together: run the SCC pass once over the valid links plus every candidate stamp. A candidate stamp whose ends lie in one component of more than one item, or with F == S, is dropped with `legacy_stamp_ignored` (`InCycle`).

Valid links alone are acyclic, so every cycle in this graph contains a stamp. Every edge of a cycle lies inside one component, so dropping each stamp inside a component removes every cycle, and never a `replaces` link. The remaining stamps are **accepted**.

**Step 5: the links.** The links are the valid `replaces` links plus the accepted stamps, deduplicated by (from, to): a valid link and an accepted stamp between the same pair cannot both exist, by the `AlreadyLinked` rule. For each node the build stores:

- `out[i]`: the items i replaces;
- `in[i]`: the items that replace i.

**Step 6: derived facts**, computed once:

- `replaced[i] = len(in[i]) > 0`; `Current = !replaced`.
- **Current successors.**
  1. Let T be a topological order of the links oriented from the replaced item to its replacer (O → N for each link N → O), so every item comes after everything it replaces. This is the orientation `History` uses; it comes from Kahn's algorithm over the acyclic links.
  2. Walk T backwards, so every replacer is visited before the items it replaces. At each node i, `succ[i]` is `{i}` if i is current, else the union of `succ[j]` for j in `in[i]`. Every j in `in[i]` is a replacer of i and has already been visited.
  3. Each set is stored as a sorted, deduplicated slice of node numbers and computed once per node, so the cost is proportional to links times the number of forks.
  4. The internal set of a current item holds the item itself. `CurrentSuccessors` returns the empty slice for a current item, and `succ[i]` for a replaced one.
- **History components.** Union-find over the links, treated as undirected.
- **`replace_fork`** (warning). For every replaced item whose `succ` has more than one member, `Item` is the replaced item, and `Successors` its current successors in item order (section 3.3).

**Step 7: `relates`.** For every item H and each distinct folded id in `H.Header.Relates`, in field order, `resolve(H, id)`:

- `One(T)` with `T != H`: an outgoing related link H → T, whatever T's section. A personal item may name shared items (spec 5.2); a shared holder only ever resolves among shared items.
- `One(H)` (self): ignored, no finding.
- `Many`: not resolved, no finding here (`check` reports `duplicate_id`).
- `None`: `personal_reference` or `dangling_link`, by the rule of step 1.

**Step 8: findings** are collected in one slice. `Findings()` returns them sorted by the holder's section, then path, then field, then the written id, so the result does not depend on map order. `check` re-sorts all findings by its own total order (architecture 13).

### 3.3 Orders

One comparator, **item order**, is used wherever the spec says "oldest first, then id":

1. `CreatedKnown` first (known dates before unknown), then `Created` ascending;
2. then `Key`;
3. then `Path`.

| Method | Order |
|---|---|
| `Replacement(x)` | `in[x]` in item order (spec 7.1 "oldest first, then by id"). |
| `CurrentSuccessors(x)` | `succ[x]` in item order. Empty when x is current. |
| `Replaces(x)` | `out[x]` in item order. |
| `History(x)` | The component of x, topologically sorted so an item follows everything it replaces (an edge O → N for each link N → O). Among items whose predecessors are all placed, the smallest in item order comes next: Kahn's algorithm with a priority queue. This is the exact order of spec 7.2. A component of one item is `[x]`. |
| `RelatesTo(x)` | Field order, as written (spec 7.1 lists "the items it names"). |
| `NewerRelated(x)` | The current items that hold a resolved `relates` link to x, other than x. Newest first: `Created` descending, unknown last, then `Key`, then `Path`. |
| `Effect` lists | Item order. |

**`NewerRelated`: the reading of "newer".** Spec 7.1 says "the newer current items that name it", and architecture 4.3 fixes "current items naming it, newest first". This design follows the architecture: there is no date filter, and "newer" names the usual case, not a condition. A hand-edited older item that names a newer one is still shown, because hiding a doubt is worse than a label that is slightly off. If @architect reads spec 7.1 as a filter (created after x), it is one added comparison here and one test.

Related items that are replaced are not dropped from `RelatesTo`: spec 7.1 marks them `replaced`, and `render` shows the marker.

### 3.4 `Unresolved` and `Shown`

Spec 5.2: "a command prints a link's target only if it exists in its view. Others are counted, never named".

- `Shown(x, f)` is the ids of x's field f, deduplicated by fold, for which `byKey` holds any item in the input. This is the command's view, not the holder's: in the personal view, a shared item's link to a personal id is printed by `get --personal`, while the link still does not count (step 1). Each id is printed as the file writes it.
- `Unresolved(x)` is the number of distinct folded ids across x's `replaces` and `relates` with no item in the input. An id named in both fields counts once.

So in the shared view, a shared item's link to a personal id is never shown, and counts in `unresolved` exactly like a missing id (scenario 6).

### 3.5 `Effect`

`Effect(items, target)` runs `Build(items)`, then `Build` on items without target (the same slice minus one pointer), and compares the two graphs:

- **`CurrentAgain`:** every item other than target that is replaced in the first graph and current in the second.
- **`Dangling`:** every item H other than target, current or replaced, that in the first graph holds a `replaces` or `relates` entry for which `resolve(H, id)` was `One(target)`, and in the second graph resolves to `None`. Each link is resolved in H's own view (architecture 7.3).
  - An invalid `replaces` link to target counts: after the delete it names a missing item.
  - A link that was already `None` or `Many` before is not listed, by construction.
- **Masked stamps.** If B validly replaces A and A also carries a stamp naming C, B's link masks the stamp. Without B, the stamp is accepted, so A stays replaced (by C) and is not current again. The comparison gets this right because both graphs are built by the same rules.

Two builds over 5,000 items cost a few milliseconds. `delete`'s preview and its confirmed run both call `Effect` on their own scans.

### 3.6 State, concurrency and edge cases

A `Graph` is immutable after `Build` and safe for concurrent reads. Every method takes an item that was in the input. For any other pointer it returns the zero value: false, nil or 0. A test checks this, so a caller mixing two scans fails visibly in tests rather than silently.

| Case | Result |
|---|---|
| `replaces: [A, a]` (same id, two cases) | One link (dedupe by fold). |
| A replaces A | `replace_cycle`; A stays current. |
| A ↔ B | Both links ignored, two `replace_cycle` findings, both current; `History(A)` is `[A]` and terminates (scenario 12). |
| Two files with id X | Every `replaces` link to X is `replace_invalid` (`Ambiguous`); `relates` to X does not resolve. |
| A stamp naming a missing file | `legacy_stamp_ignored` (`Missing`); the note stays current (scenario 11). |
| B replaces A, and B is stamped `superseded-by A` | The stamp A → B closes a cycle and is dropped; A is replaced, B current; `History` is `[A, B]` (scenario 11). |
| A note with a valid successor B and a stamp naming C | The stamp is `AlreadyLinked`; B is the one successor (scenario 11). |
| B and C replace A, then D replaces B | A's current successors `[C, D]` in item order; `replace_fork` on A remains (scenario 12). |
| A verified item replaced by a provisional one | `replace_invalid` (`Standing`); the old item stays current (scenario 10). |
| A decision without `decided` (standing 1) replaced by a verified learning | `replace_invalid` (`DecisionKind`). |
| A source replaced by a note | `replace_invalid` (`SourceKind`). |
| A replaced item replaced again | Valid on disk: a second link makes a fork. `add` refuses it before writing through `CheckNewReplaces`. |

## 4. Dependencies

- **Uses:** `item` (`Item`, `Header.Standing`, `Header.LegacyStamp`, `Key`) and `errs` (for `CheckNewReplaces` only). Architecture 3 lists `graph` → `item`; adding `errs` is the usual leaf import every package has.
- **Used by:**
  - `search`: `Current`, `Replacement`, related lines;
  - `render` and the reading commands: `get` (`Replacement`, `CurrentSuccessors`, `History`, `Replaces`, `Shown`, `Unresolved`, related lines), `list` and `prime` (`Current`);
  - `check` (`Findings`);
  - `writes`: `CheckNewReplaces` for `add` and `import`; `Current` for `verify`; `Effect` for `delete`.

## 5. Test plan

Pure unit tests over in-memory items, built by a helper `mk(id, section, kind, standing, created, replaces, relates, stamp)`.

| Test | What it proves | Spec |
|---|---|---|
| `TestChainCurrentAndSuccessors` | A ← B ← C: only C current; `Replacement(A) = [B]`; `CurrentSuccessors(A) = [C]`, `CurrentSuccessors(B) = [C]`, `CurrentSuccessors(C) = []`; `History(C) = [A, B, C]`. The complete successor sets of the chain, a diamond (B and C replace A, D replaces both: A, B and C give `[D]`) and a fork (B and C replace A: `[B, C]`). | 4.4, scenario 4 |
| `TestValidityRules` | Each rule of 4.2: standing, decision kind, source kind, other section, ambiguous target; a missing target is `dangling_link` and never `replace_invalid`. | 4.2, scenario 10, 29 |
| `TestVerifiedReplacedByVerifiedStaysVerified` | Replacement changes no recorded status; the provisional link to it is refused and reported. | scenario 10 |
| `TestCyclesIgnoredHistoryEnds` | Self-link, two-cycle, three-cycle with a tail; all cycle links ignored, items current, `History` terminates and lists each once. | 4.2, scenario 12 |
| `TestForks` | B and C replace A, then D replaces B, then E replaces C and D: the forks, their successors and the findings at each step. | 4.4, scenario 12 |
| `TestLegacyStamps` | Every case of scenario 11 and of section 3.6, including two stamps closing a cycle together (both dropped) and a stamp in a cycle with a valid link (only the stamp dropped). A personal stamp with one personal target and a shared note of the same stem is accepted in the personal view; with two personal targets it is `Ambiguous`. | 4.3, scenario 11 |
| `TestFactoryNotesGraph` | Factory-shaped items: a stamped note hidden, its successor current, the decision without decider unchanged in standing. | 13, scenario 21 |
| `TestDuplicateIdsInvalidateLinks` | Two files with one id and with ids differing only in case: links to them invalid, `relates` unresolved. | 2.3, scenario 13 |
| `TestSectionsAndViews` | Shared holders resolve among shared items only, also in the personal view; a personal `relates` to a shared item resolves; a personal `replaces` to a shared item is invalid (`OtherSection`); an id in both sections is `Many` for a personal holder; `personal_reference` only in the personal view, `dangling_link` with no target in the shared view. | 5.2, scenarios 6, 7 |
| `TestShownAndUnresolved` | Shared view: a link to a personal id is not shown and counts 1; personal view: shown. An id named in both fields counts once. | 5.2, 7.2, scenario 6 |
| `TestRelatedOrders` | `RelatesTo` in field order; `NewerRelated` newest first, unknown last, then id; replaced holders excluded; self-links ignored. | 7.1, scenario 2 |
| `TestHistoryOrder` | Ties oldest first, then id; unknown dates last; a diamond (B and C replace A, D replaces both) gives `[A, B, C, D]`. | 7.2, scenario 12 |
| `TestEffect` | Scenario 29: deleting C makes B current again and leaves the relating note dangling; an already-unresolved link is not listed; deleting B with a masked stamp on A keeps A replaced; a personal holder is listed only when personal items are in the input; deleting A while B says `replaces: [A]` lists B as dangling. | 8.6, scenario 29, architecture 7.3 |
| `TestCheckNewReplaces` | `not_found`, `ambiguous_id`, each `invalid` rule, an already-replaced target with its current successors; one success. The precondition: a fixture where an existing item holds a dangling `replaces` naming an id, and `writes.md`'s id draw refuses that id (the cycle it would close never forms). | 4.2, 8.2, scenarios 10, 14, 19 |
| `TestForeignItemZeroValues` | Methods given an item not in the input return zero values. | — |
| `TestDeterministicFindings` | The same items in shuffled input order give identical findings and orders. | 6.1 "order is fixed" |
| `FuzzBuild` | Random links, stamps, sections and duplicates: `Build` never panics, every `History` terminates and contains its item once, accepted links are acyclic, and every current item has `CurrentSuccessors` empty. | architecture 14 |
| `BenchmarkBuild5000` | 5,000 items with chains, forks, stamps and relates; reported with the architecture 8.6 benchmarks. | 16 speed |
