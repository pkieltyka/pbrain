# Plan: bounded catch-up inside `search --semantic`

Issue #26, queued behind #24 (`pbrain sync`) and #25 (CRC-32C freshness). Owner's decision of
2026-11-21. The spec carries the design: functional spec §7.1 ("Catching up", "Sameness"), §9.4,
§15 scenario 42, §16; architecture §8.7 ("Catch-up in a search"), §10.4, §16 decision 30, §17;
`components/semantic.md` ("Catch-up in a search" and its tests); `components/store.md`
(`Files.TryLock`); `components/cli.md`; `components/skill.md`.

Rechecked against #25's code on master (`5a90234`). The names the design uses are the code's:
`Check{CRC, Size}`, `ContentCheck`, `storedItem{id, check}`,
`putItem(ctx, path, content, check, passages)`, `work{content, check}` and the manifest's
`CRC32C`. One thing differed and is fixed in the spec: #25 says that only the build computes
`ContentDigest`. A catch-up computes it too, for each item it takes on.

## Decision

When a section has a usable index and the model answered the query, a search with `--semantic`
embeds and saves the items that index does not cover, then answers with them.

- Bounds: at most 20 uncovered items in the view, at most 20 passages to embed, and no embedding
  call started later than 2 seconds after the first lock is taken.
- Over a size bound it writes nothing. When the time runs out it saves what is finished.
- It takes `index/lock` by one attempt and never waits.
- A first index, an index for another model, an unreadable index, a model download and anything
  larger stay with `pbrain sync`.
- A search without `--semantic` is unchanged.

## Interfaces

```go
package sys

var ErrLocked = errors.New("lock is held")

type Files interface {
    // … the existing methods …
    // TryLock opens the lock as Lock does and makes one attempt. ErrLocked when it is held.
    TryLock(dir Dir, name string) (io.Closer, error)
}
```

```go
package semantic

const (
    catchUpItems    = 20
    catchUpPassages = 20
    catchUpBudget   = 2 * time.Second
)

// CatchUp returns only the context's error. Call it once, after Attach and before Score.
func (s *Source) CatchUp(ctx context.Context) error
```

No change to `search.Semantic`, to `search.Scores`, to any notice or to any JSON field.

## Files

- `internal/sys/sys.go`, `sys_unix.go`: `ErrLocked`, `Files.TryLock`. Share the open and the
  owner-only check with `native.Lock` (`sys_unix.go:459`); `EWOULDBLOCK` becomes `ErrLocked`.
  Every test double embeds `sys.Files`, so none needs a new method.
- `internal/semantic/index.go`: `openIndex` also returns the manifest it read.
- `internal/semantic/source.go`: `openedIndex` keeps that manifest; `CatchUp`.
- `internal/semantic/build.go`: the build's steps for one item (reuse by digest, embed, cache
  write, `putItem`) and `publish` are called by both. Move what `CatchUp` needs out of
  `builder.run` and `builder.flush` into functions both use; do not copy them.
- `internal/semantic/passages.go`: the comment on `ContentDigest` names the catch-up as its
  second caller. `contentCalls` already counts the calls for the tests.
- `internal/semantic/cache.go`: a way to ask whether the cache folder exists, so that a search
  never creates it (`dirLocked("", true)` would).
- `internal/cli/read.go:292`: after `src.Attach(…)`, `if err := src.CatchUp(ctx); err != nil`
  returns that error.
- `internal/cli/semantic.go` `progressMessage`: the waiting text becomes
  `<section>: waiting for another pbrain command` (operator's decision), since the holder can be a
  search. #24 sets it to `waiting for another pbrain sync` first; this replaces that.
- Text: `internal/skill/topics/semantic.md` (the text of `components/skill.md`, 289 words),
  `README.md:46` ("Search never downloads a model or builds an index").
- Scenario scripts: 35 expects `semantic_partial` after one `add`; that state now needs 21 added
  items or a held lock. New script for scenario 42.

## Order, with the proving test of each step

1. `Files.TryLock`: `TestLockWaitPolicies` (`store.md`).
2. `openIndex` returns its manifest; the shared per-item functions. The existing build tests
   must pass unchanged.
3. `CatchUp`, the gap and the plan, no writing yet: `TestCatchUpBounds` (the cases that write
   nothing), `TestCatchUpNeverCreates`.
4. The lock, the manifest comparison, embedding, `putItem`, publication: `TestCatchUp`,
   `TestCatchUpLock`, `TestCatchUpFailures`, the rest of `TestCatchUpBounds`.
5. `cli`: `TestCLISemanticAndOutline` (the call order and scenario 42 through the command);
   `TestSemanticNoticeTable` (the waiting text).
6. Scenario scripts 35 and 42, the skill text and its word count, the README line.
7. Measurement, when heavy runs are allowed: a search after one added note and a search at the
   full bound of 20 passages, on the 5,000-item brain, added to `internal/cli/README.measure.md`.

## Error cases

None reaches the user as an error. Each leaves the items uncovered, and `Score` counts them in
`Missing`, so the search prints `semantic_partial`.

| Case | What `CatchUp` does |
|---|---|
| The source is not ready | returns at once; nothing is read or written |
| More than 20 uncovered items, or more than 20 passages to embed | returns; no lock, no write |
| `TryLock` gives `ErrLocked` or any other error | skips that section |
| The manifest changed since the index was read | releases the lock, skips that section |
| The model task failed | returns before any lock |
| A vector fails its test, or `Embed` fails | ends that section; items already replaced are published |
| The cache is exposed below its root | ends the catch-up |
| Publication fails | not an error; the replaced items are still scored in this search |
| The time runs out | starts no further call; publishes the items finished |
| The context ends | returns the context's error; nothing more is published |

## Rejected

- **Catching up inside `Score`.** `search` calls `Score`, so a write would hide behind a method
  that the search package believes only compares. The call in `cli` keeps the write visible and
  the seam unchanged.
- **Waiting a short time for the lock.** The decision is "only if it is free". A wait would also
  put a second search behind the first one's 2 seconds.
- **`Lock` with a context that has already ended**, as a try-lock. It works today, by the order
  of two lines in a loop, and nothing would keep it working.
- **The build's batches of eight.** One call could overrun the 2 seconds by eight passages, about
  9 seconds with EmbeddingGemma 2.
- **Catching up the first 20 of a larger gap.** Every search after a large pull would pay
  2 seconds and still print the notice. The decision leaves larger gaps to `pbrain sync`.
- **A second scan under the lock**, as a build takes. Catch-up deletes nothing except the old row
  of an item it replaces, so a stale scan can only produce an entry that the next command's check
  rejects.
- **A notice when a catch-up worked.** It would add a line to most searches that follow a write.
- **Bounds per model.** A new constant in every model row, for one caller.

## Risks and gaps

- Nothing here is measured. The 0.3 seconds for one note and the 2 seconds at the bound are sums
  of parts measured apart.
- One passage a call may be slower for bge-small than batches of eight. Step 7 shows it; if it
  matters, the batch size is the only thing to change.
- With EmbeddingGemma 2 the 2 seconds allow two or three passages.
- Where the lock can be taken and the write fails, each search embeds the same personal items
  again.
- There is no setting that turns catch-up off.
- Tests that expect `semantic_partial` after a single `add` or edit change:
  `internal/cli/semantic_test.go:77` ("1 item is not in the index"), and the cases of scenario
  35's script that search after an `add` or an edit with an index present. Its two
  `semantic_partial` cases with no personal index stay as they are. The `ContentDigest` count in
  `internal/semantic/source_test.go:340` stays valid only for a search with nothing to catch up.

## Open questions

| Question | Owner |
|---|---|
| Is a switch to turn catch-up off wanted, for automation that expects a search to write nothing? The design has none. | the owner |
