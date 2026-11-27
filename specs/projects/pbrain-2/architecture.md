---
status: draft
---

# Architecture: pbrain-2

This document says how pbrain-2 is built. It is a delta on the first project's architecture (`specs/projects/pbrain/architecture.md`, "the pbrain architecture" below, cited by section) and implements `functional_spec.md` in this folder ("the spec"). Everything the pbrain architecture decides stays in force unless this document says otherwise: one Go module, no cgo, one short process per command, no daemon, files reached only through open folder handles, one renderer for every output.

It is the first of two parts. Section 13 lists five component documents that follow it; this document holds the data model, the packages and the interfaces between them, and the solutions to the hard problems.

## 1. Decisions at a glance

| Area | Decision | Why |
| --- | --- | --- |
| Memory storage | Per section: one ordinary SQLite database on disk, in WAL mode, opened by path, written in one transaction per command (the owner's decision of 2026-11-26). | Measured: a store costs about 5 ms whatever the size, against 170 ms at the caps for a file replaced whole (section 4.6). It is the one place where pbrain opens a file by path, and section 6.3 says how that is contained. |
| Memory location | The per-user state root, shared memory under the project, personal memory under the personal section's place. | Memories are originals; the cache is disposable (spec 4.1). |
| Process model | Unchanged: one process per command. Hooks are ordinary short runs of the same binary. No resident helper. | Nothing to start, stop or leave stale. The spec left the in-memory helper out of the first version. |
| The previous answer | Read from the host's own session file at the moment of the request, by a per-host reader that accepts only tested host versions and fails closed. | No per-turn work and no second copy of answers. The file's format is not a documented contract, so the reader is narrow and version-gated (section 7). |
| Who asked | Read from the same session file: the answer to a dialog whose question pbrain's skill fixed. | "Observed" needs a positive event. A hook for it would add an event; the record already holds it. |
| Promotion | The existing guarded item writer, entered with a reserved id and frozen bytes; the token is spent durably before publication. | One writer for items; at most one item per token (spec 10.1). |
| Documents | A second admission path beside items: a `doc` model, a scan in `store`, a merge in `search`. A document is never turned into an item. | A synthetic item would collide on ids and enter the replacement graph. |
| Indexes | One stable SQLite database per checkout and index kind, opened and updated in place, in the per-user cache; the personal section's in a folder of its own. A build for another model or profile is staged inside the same database and takes over, with the superseded rows deleted, in one transaction. No in-memory image, no manifest, no file per build or per model. | The owner's decisions. Each commit is a usable checkpoint, a build resumes, nothing is left to clear (section 10.4). Vectors are shared through the existing cache by content. |
| Where pbrain keeps things | Nothing in the working tree. Rebuildable data in the cache, memories and originals in the state folder, every new thing below a `.pbrain` folder there, keyed by project, checkout or personal section and bound to the folder's identity. | The owner's decision (spec 12). |
| Taking turns | No lock files (the owner's decision of 2026-11-27). A writer holds an open SQLite write transaction on a small "turn" database in the state root, one per thing that needs turns: a section, a host settings file, an index, a model download. Nothing is ever stored in a turn database. | Released by the operating system when a process dies, a bounded wait, one mechanism. A cache purge cannot remove it. Every item write therefore needs the state root (section 6.4). |
| pbrain's own records | Rows in one records database in each root: which project has which memory, which folder has which id, what `hooks install` inserted, that an old folder was moved. | No marker or record files, and each record is one transaction. |
| Patches | A small line diff of pbrain's own, checked in tests against `git apply`. | No new dependency for one output format. |
| A person's view of memory | Lists over the same rows: `memory log`, `sessions` and `highlights` as ordinary reads with a keyset cursor; labels in a table of their own; a screen, `memory browse`, in one package that the rest of pbrain does not import. | Agents fill memory on their own and a person reviews afterwards (spec 5.5 to 5.7). The screen adds no ability the commands lack. |
| Dependencies | Four added, for the screen only, by the owner's choice of 2026-11-27: `charm.land/bubbletea/v2`, `charm.land/bubbles/v2`, `charm.land/lipgloss/v2`, `charm.land/glamour/v2`. They build without cgo; the release audit does not pass with them linked into pbrain, and how the screen is shipped is an open decision for the owner (section 5.5). Nothing else is added. | The first project's rule (pbrain architecture 15) was none; this is the owner's exception. |

## 2. System shape

Three new pipelines sit beside the existing one. Each starts in `cli` and ends in `render`, as every command does.

```
store / recall / forget / promote
  cli ─► brain (guard, section, author) ─► memory (state root, lock, admit or read) ─► render
                                              │ promote only
                                              ▼
                                           store.Tx (the item writer) or doc writer

hook session-start / pre-compact, hooks install
  cli ─► hosts (parse the host's event) ─► brain ─► memory (session record, page) ─► host's reply form

docs search / get / list / sync / check / inventory
  cli ─► brain (folders, project root) ─► store (items) + store.ScanDocs (documents)
        ─► graph (items only) ─► search (one merged order) ─► semantic (two indexes) ─► render
```

## 3. Packages

New and changed packages, in the dependency order of the pbrain architecture (section 3): a package imports only packages below it.

| Package | New or changed | Responsibility | Imports (of pbrain's own) |
| --- | --- | --- | --- |
| `cli` | changed | New commands: `store`, `recall`, `forget`, `promote`, `memory init`, `memory log`, `memory show`, `memory sessions`, `memory highlights`, `memory browse`, `hook`, `hooks`, `docs …`. New operand form for arrays in the catalog. | all |
| `browse` | new | The screen of `memory browse`: a Bubble Tea program over the lists of `memory`. The only package that imports the four Charm libraries, and imported only by the `memory browse` command. It gets its data and its actions through two small interfaces that `cli` implements, so it imports neither `cli`, `store` nor `brain`. | `memory` (types), `render`, `errs` |
| `hosts` | new | The host-neutral types of an adapter: an event, a reply, a session reader. The table of supported hosts and versions. Installing and removing a host's settings. | `memory`, `guarded`, `sys`, `errs` |
| `hosts/claude` | new | Claude Code: parsing its hook events, forming its replies, reading its session file. Imported only where the host table is built, as model adapters are. | `hosts`, `errs` |
| `check` | changed | Document findings beside item findings. | adds `doc` |
| `semantic` | changed | The index as an on-disk database opened in place, for items, documents and the personal section, all in the cache; a neutral entry in front of the passage and vector core; no image, manifest or generation. | adds `doc`, `guarded` |
| `embed` | changed | `Model.CPURate`: the model's measured processor rate, for the estimate in the `cpu_fallback` warning (10.4). | none |
| `search` | changed | Matching and ranking over items and documents together. | adds `doc` |
| `memory` | new | The memory record, the database, admission, retention, replay, quota, labels (tags and keywords), recall order, the lists a person reviews with, the count of dropped rows, the task page, promotion previews and tokens. The only package besides `semantic` that imports the SQLite driver. | `guarded`, `query`, `item` (actors and the fold only), `secretscan`, `sys`, `errs` |
| `store` | changed | `ScanDocs` and `InventoryDocs`; `CreateDoc`; originals in the state root; the section's turn in `Begin`; `Migrate`, the move of an old `.pbrain/`; inputs refused inside both roots. | adds `doc`, `guarded` |
| `brain` | changed | `[docs]` and `recall` in the configuration; the project root; the document folders and what is pruned from them; the memory project key. | unchanged |
| `doc` | new | The document model: its address, its header, its title and body, its line arithmetic. | `outline`, `errs` |
| `difflines` | new | A line diff and a unified patch writer. | none |
| `guarded` | new | The two per-user roots and the rule that keeps them apart; the records database and places; guarded child folders; `OpenDB`, the one function that opens an SQLite file by path, with its checks and the lock mode; `Turn`, taking a turn. `semantic.OpenCache` becomes a caller. | `gitx`, `sys`, `errs` |
| `sys` | changed | `Dir.RenameNew`: a rename within one folder that fails if the new name exists, on the no-replace rename the package already has for Linux and macOS. | none |

`memory` does not import `store` or `brain`: `cli` resolves the brain, the section, the author and the folder handles, and passes them in, exactly as it joins `search` and `semantic` today. `buildcheck`'s import-rule test gains these rows.

## 4. Memory: the data model

### 4.1 The record

```go
package memory

type Kind string      // "handoff" | "outcome" | "blocker" | "statement"
type Term string      // "short-term" | "long-term"
type Capture string   // "authored" | "verbatim" | "checkpoint" | "report"
type Basis string     // "observed" | "claimed"
type Allocation uint8 // none (short-term), ordinary, person

type Row struct {
    ID        string    // "mem_" + 32 hex
    Kind      Kind
    Learned   bool      // intent: learned; only with Kind statement and Term long-term
    Term      Term
    Capture   Capture
    Fact      string    // set for Capture report: "check-passed" | "check-failed" | "handoff-delivered"
    Task      string
    Run       string    // the supervisor's run, when given
    Tags      []string  // at most TagMax, folded, sorted, without repeats; from the caller
    Keywords  []string  // at most KeywordMax; derived by pbrain at admission (4.3), never supplied
    Ref       string
    SaidBy, AskedBy, RecordedBy string
    Source    string    // producer token
    Observed, Requested, Received time.Time
    Expires   time.Time // zero: no end
    Session, Turn, RequestTurn string
    Harness, HostVersion, Model, Effort, SourceBranch, SourceCommit string
    RecordedBranch, RecordedCommit string
    RecordedDirty bool
    Basis     map[string]Basis // one entry per provenance field that is present
    Alloc     Allocation
    Bucket    string    // the short-term cap it counts in (4.5)
    TextBytes int
    TextSHA   [32]byte
    Event     string    // the caller's UUID, when given
}

const (
    TextMax       = 65536
    CheckpointMax = 2048
    MetaMax       = 8192 // UTF-8 bytes of all metadata values of one row together; tags count, keywords do not
    TagMax        = 8
    KeywordMax    = 5
)
```

A typed report has no text: its wording is rendered from `Fact`, `Source` and `Task` on output, so nothing a caller wrote is stored for it. `MetaMax` is a derived limit: the spec bounds each field in characters, and a row's metadata is also refused with `too_large` above 8,192 bytes in all, so that no row can cost more than 8 KiB beside its text.

### 4.2 Files

One memory folder per physical section, both in the state root and neither in a working tree (section 6.2). The shared one is `<state>/.pbrain/memory/v1/<project-key>/<enrolment-id>/`; the personal one is `<state>/.pbrain/memory/v1/personal/<personal-id>/<enrolment-id>/`. Which enrolment folder is current is a row in the state's records database (6.3).

```
<memory folder>/                 mode 0700, created by pbrain
  memory.db                      the database, mode 0600
  memory.db-wal, memory.db-shm   SQLite's own side files, present while it is in use
```

Everything except the database and its two side files is reached through `sys.Dir` handles, as every pbrain file is. The folded `.pbrain` component keeps `store.OpenInput` refusing all of it as input.

### 4.3 The schema

```sql
PRAGMA application_id = 0x70627231;   -- "pbr1": marks the format, nothing more
PRAGMA user_version   = 1;

CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL) STRICT, WITHOUT ROWID;
-- rows, shared memory:   kind = "memory", scope = "shared",   project = the project key, incarnation = the enrolment id
-- rows, personal memory: kind = "memory", scope = "personal", incarnation = the personal enrolment id, personal = the personal id   (no project: see 6.3)

CREATE TABLE memory (
  rowid INTEGER PRIMARY KEY AUTOINCREMENT,   -- the admission sequence of this enrolment: it only rises and is never reused
  id TEXT NOT NULL UNIQUE,
  kind TEXT NOT NULL, learned INTEGER NOT NULL, term TEXT NOT NULL, capture TEXT NOT NULL, fact TEXT,
  alloc INTEGER NOT NULL, bucket TEXT,
  task TEXT, run TEXT, ref TEXT, said_by TEXT, asked_by TEXT, recorded_by TEXT NOT NULL, source TEXT NOT NULL,
  observed INTEGER NOT NULL, requested INTEGER, received INTEGER NOT NULL, expires INTEGER,
  session TEXT, turn TEXT, request_turn TEXT,
  harness TEXT, host_version TEXT, model TEXT, effort TEXT, source_branch TEXT, source_commit TEXT,
  recorded_branch TEXT, recorded_commit TEXT, recorded_dirty INTEGER,
  basis TEXT NOT NULL,                 -- a JSON object: field name to "observed" or "claimed"
  keywords_v INTEGER,                  -- the version of the keyword rule that was applied; NULL when none was (a report)
  meta_bytes INTEGER NOT NULL,
  text BLOB NOT NULL, text_bytes INTEGER NOT NULL, text_sha BLOB NOT NULL
) STRICT;
CREATE INDEX memory_task    ON memory(task, observed DESC);
CREATE INDEX memory_term    ON memory(term, alloc, received);
CREATE INDEX memory_bucket  ON memory(bucket, received) WHERE bucket IS NOT NULL;
CREATE INDEX memory_expires ON memory(expires) WHERE expires IS NOT NULL;
CREATE INDEX memory_recv    ON memory(received, id);                          -- the log, and its cursor
CREATE INDEX memory_run     ON memory(run, received, id) WHERE run IS NOT NULL;
CREATE INDEX memory_session ON memory(session, received, id) WHERE session IS NOT NULL;

CREATE TABLE receipt (
  event TEXT PRIMARY KEY,              -- the caller's UUID, 36 characters
  digest BLOB NOT NULL,                -- 32 bytes
  mem_id TEXT NOT NULL,                -- 36 characters
  observed INTEGER NOT NULL,           -- the receipt lives until observed + 14 days
  gone INTEGER NOT NULL                -- the memory was forgotten, dropped or expired
) STRICT, WITHOUT ROWID;

CREATE TABLE approval (                -- one row per observed dialog answer that was used; content-free
  session TEXT NOT NULL, dialog_turn TEXT NOT NULL,
  text_sha BLOB NOT NULL, term TEXT NOT NULL, section TEXT NOT NULL,
  answered INTEGER NOT NULL,           -- kept until answered + 14 days, also after a forget
  PRIMARY KEY (session, dialog_turn)
) STRICT, WITHOUT ROWID;

CREATE TABLE promo (
  token_sha BLOB PRIMARY KEY,          -- SHA-256 of the token
  sources TEXT NOT NULL,               -- memory ids, sorted, one per line
  target TEXT NOT NULL,                -- JSON: kind, section, checkout identity, reserved item id or document path
  draft BLOB,                          -- the frozen bytes; NULL once published
  draft_sha BLOB NOT NULL, draft_len INTEGER NOT NULL,
  expires INTEGER NOT NULL,            -- preview time + 10 minutes
  spent INTEGER NOT NULL, published INTEGER NOT NULL
) STRICT, WITHOUT ROWID;
```

Times are whole seconds since the epoch, UTC. The text is a BLOB so that SQLite stores the bytes it was given and nothing else.

```sql
CREATE TABLE word (
  word TEXT NOT NULL,                  -- one distinct word of the row, as query.Scanner yields it, folded by item.Fold
  mem INTEGER NOT NULL REFERENCES memory(rowid) ON DELETE CASCADE,
  PRIMARY KEY (word, mem)
) STRICT, WITHOUT ROWID;
CREATE INDEX word_mem ON word(mem);        -- so that deleting a row does not scan the table
CREATE INDEX receipt_mem ON receipt(mem_id);

CREATE TABLE label (
  label TEXT NOT NULL,                 -- folded; a tag (ASCII, at most 32 bytes) or a keyword (one word of 3 to 32 characters, any script)
  mem INTEGER NOT NULL REFERENCES memory(rowid) ON DELETE CASCADE,
  origin INTEGER NOT NULL,             -- 0 = a tag the caller gave, 1 = a keyword pbrain derived
  PRIMARY KEY (label, mem, origin)
) STRICT, WITHOUT ROWID;
CREATE INDEX label_mem ON label(mem);

CREATE TABLE dropped (                 -- numbers only: how many short-term rows a count limit removed
  hour INTEGER NOT NULL,               -- the hour in which the removed rows had been received, as seconds since the epoch
  reason INTEGER NOT NULL,             -- 1 = the bucket's count, 2 = the section's rows or bytes
  n INTEGER NOT NULL,
  PRIMARY KEY (hour, reason)
) STRICT, WITHOUT ROWID;
```

`label` holds both kinds of label apart by `origin`, written in the row's own transaction and removed with the row. A filter by label is one look-up by its primary key for each label asked for, whatever the origin. `dropped` is the whole record of a removed row: a count against an hour. Its rows are deleted 14 days after their hour by the next write, and a read ignores any that are older, so a count never outlives its 14 days for want of a write.

**Keywords** (spec 3.6) are computed inside the store's transaction, after the secret scan and after expired rows are deleted, so "the live memories of the section" is exactly what the transaction sees:

```
keywords(text):                                   # version 1
  words := the distinct words of text, as query.Scanner yields them (already folded), each with its count tf
           # the scanner reuses its buffer: each word is copied before it is counted
  drop a word in the stop list (a fixed list compiled into pbrain), of fewer than 3 or more than 32 characters, or of digits only
  cand := the 64 words with the highest tf; ties by the word, ascending
  N := SELECT count(*) FROM memory                # before this row is inserted
  for w in cand: df := SELECT count(*) FROM word WHERE word = w
                 weight := tf * (1 + floor(log2((N + 2) / (df + 1))))     # whole numbers only; the division is integer division
  return the 5 with the highest weight; ties by the word, ascending
```

Whole numbers keep the answer identical on every platform, which a floating-point logarithm would not promise at a tie. Everything down to `cand` is worked out before `BEGIN IMMEDIATE`; only `N` and the 64 counts need the transaction. The limit of 64 candidates bounds that work to 64 counts over the word index, each over at most the section's rows, whatever the size of the text. That bound is not small: at the limits it is up to 704,000 index entries, and @lead-dev's scratch probe of such counts alone, not in pbrain, took 29.5 ms at the median and 31.1 ms at the 95th percentile. Phase 1 measures it in pbrain with five writers (11.3). If it does not fit the store's budget, an exact count for each word, kept up to date in the same transaction as the word rows, replaces the 64 counts; that is not built before the measurement asks for it. The stop list and the rule are versioned together by `keywords_v`; a later version applies to new rows only, and nothing recomputes old ones. A replayed keyed store returns the first row, with its keywords as they were.

`word` is the recall index (section 5.3). It is written by pbrain, with pbrain's own tokenizer, in the same transaction as its row, and goes with the row when the row is deleted. It is not SQLite's full-text search, whose tokenizer does not fold as pbrain does. For a typed report the words are those of its rendered wording.

The `application_id` and `user_version` say what format a file has. They do not say whose it is: two memory databases of two projects carry the same values. What pbrain checks before it uses a database is the `meta` rows, the place and the section it expects (section 6.5). A `user_version` above 1 fails with `invalid_config` and is left alone; a migration to a later version runs in one transaction on the first write after an upgrade.

### 4.4 The connection

Every command opens the database once, does its work, and closes it. The settings are fixed by pbrain and none comes from a caller, the repository or the environment:

| Setting | Value | Why |
| --- | --- | --- |
| `journal_mode` | `WAL`, set once when pbrain creates the database; on an existing one it is only read and must be `wal` | Readers never wait for a writer; one record is written, not the file |
| `synchronous` | `FULL` | A commit that returned is on disk. Measured cost against `NORMAL`: none that shows (4.6) |
| `busy_timeout` | the smaller of 2,000 ms and the time left to the caller | Then `memory_busy` |
| `temp_store` | `MEMORY` | SQLite creates no temporary file of its own anywhere |
| `foreign_keys` | `ON` | |
| `max_page_count` | 4 GiB of pages, provisional (4.5) | A hard ceiling on the file whatever a bug does; reaching it is `io` |
| The lock mode of the driver | conventional POSIX locks on Linux and on macOS: fixed in code with `sqlite.OFDLocking(false)` before the first open on Linux, the only mode on macOS | Section 6.5 |

Each setting is read back from the open connection before the first statement, and a mismatch fails with `io`. A write is one `BEGIN IMMEDIATE` transaction: replay check, expiry, caps, quota, the row, the receipt, all or nothing. A read is one read transaction and sees one committed state. Statements are prepared with parameters only; no text from a caller is ever part of SQL.

### 4.5 The budgets, exactly

All are enforced inside the write transaction, with `SUM` and `COUNT` over the rows, so they cannot drift.

| Content | Limit per section | When full |
| --- | --- | --- |
| Short-term rows | 1,000 rows and 8 MiB of text per section; 32 per bucket | The oldest short-term rows are deleted, by `received` then id, until the new one fits: first in the new row's bucket, then in the section. Each row removed this way adds one to `dropped` for its hour and reason; a row removed for its age does not |
| Long-term, person's allocation | 2,000 rows and 64 MiB of text | `quota_full` |
| Long-term, ordinary allocation | 8,000 rows and 256 MiB of text | `quota_full` |
| Labels | 8 tags a row, each at most 32 bytes, counted in the row's metadata; 5 keywords a row, not counted | A ninth tag is `usage` |
| Metadata | 8,192 bytes per row (`meta_bytes`) | `too_large` for that store |
| Replay receipts | 50,000 rows, each of fixed size (36 + 32 + 36 bytes and two integers) | A new **keyed** store is refused with `quota_full`; an unkeyed store and `forget` still work |
| Promotion previews | 32 live; 1 MiB per draft | A draft over 1 MiB is `too_large`; the 33rd preview cancels the oldest unspent one |
| The file | 4 GiB (`max_page_count`), **provisional** | `io`. It is a backstop, and it must stand above everything the limits allow, or a store inside its own allocation could fail on the file and the reserve for a person's requests would not be a guarantee. What the limits allow in content: 328 MiB of text, 90 MiB of metadata, 32 MiB of drafts, receipts, labels, and the word index. The word index is the large and uncertain part. For ordinary prose it is smaller than the text, but @architect-2's scratch probe of texts made only of distinct three-letter words (96 texts, 3,145,632 bytes) took 20,865,024 bytes of pages, 6.63 for each byte of text: 3,198,976 for the texts themselves and 17,661,952, or 5.61 for each byte, for the word table and its index. That projects to about 2.1 GiB for 328 MiB of such text, texts and both trees together, before metadata, labels, drafts and receipts. That is a small fixture and not a measurement at the limits; the earlier measurement, 67 MiB for 48 MiB of ordinary text, says nothing about this shape. So 4 GiB is a first figure above that projection, and phase 1 settles it (11.3): if a legal fill needs more than is reasonable for one section, the byte limits come down or the index is made cheaper, and the owner is told which |

The text quotas are the logical bytes the owner approved; SQLite's own overhead is not charged to them and is covered by the file ceiling. Each allocation is counted only from its own rows, so pressure from short-term traffic or from the ordinary allocation can never take room from the person's allocation, and `forget` never needs room.

The bucket of a short-term row is `task:<task>`, else `recorder:<name>` when a supervisor observed the recorder, else `unassigned`; each holds 32. A tag, a session and a run are never part of a bucket. The order inside a store is: delete what has expired; then the new row's bucket; then the section's rows and bytes. The allocation of a long-term row is `person` when `asked_by` was observed as a person, else `ordinary`. Both are decided once, at admission. A duplicate returns the first row and changes nothing.

Expired rows are invisible to every read at once, because every read filters on the clock, and are deleted by the next write in that section. Deleted pages are reused by SQLite; the file does not shrink by itself, and pbrain never runs `VACUUM` on a hot path. `forget` is a logical deletion, not secure erasure, and the README says so.

### 4.6 Why a database on disk: what was measured

On the team's Linux machine (NVMe, btrfs), with the pinned driver, each operation as a fresh open and close, rows with realistic metadata and texts of 300 bytes to 16 KiB:

| At | A JSON file replaced whole | SQLite on disk, WAL, full sync | SQLite in memory, image replaced whole |
| --- | --- | --- | --- |
| 500 memories, one store | 9 ms | 4.7 ms | 6 ms |
| 2,000 memories, one store | 32 ms | 4.9 ms | 20 ms |
| 10,500 memories, one store, median / 95th | 168 / 182 ms | 4.9 / 6.4 ms | 90 / 108 ms |
| 10,500, recall by a full scan of every text | 73 / 84 ms | 25 / 29 ms | n/a |
| 10,500, the task page | as recall | 0.4 ms | as recall |

These are single-writer figures from a scratch program, not from pbrain. With `synchronous=NORMAL` the store was 4.9 / 6.0 ms, so `FULL` costs nothing that shows.

@lead-dev then measured the database at the caps with nearly all 48 MiB of text and 49,000 receipts, each operation a fresh process, on the same machine:

| At the caps | median / 95th |
| --- | --- |
| One store, with the word index, unkeyed / keyed | 8.3 / 9.2 ms; 8.6 / 9.0 ms |
| Five stores at once, until all five return | 44.9 / 66.0 ms |
| Recall by a full scan of every text with pbrain's tokenizer, warm, a late match / a miss | 203.8 / 208.4 ms; 203.3 / 206.7 ms |
| The same, cold | 321 / 533 ms |
| Recall through the word index, a rare word / a miss / a word in every row | 2.0 / 2.2 ms; 1.7 / 1.9 ms; 13.7 / 14.3 ms |
| The task page | 1.7 / 1.9 ms |

So the full scan does not meet the 100 ms gate at the caps and the word index does, which is why section 5.3 uses it. One store in that run took 415 ms: commits and checkpoints have a tail, and no latency is promised for every single call. Not yet measured: queries of several common words, phrases, texts of 64 KiB made of distinct words, the whole command, and macOS. Phase 1 measures them in pbrain (section 11.3).

### 4.7 Replay

A keyed store carries `--event` and `--observed-at`. The canonical payload is what the producer asked for and nothing pbrain observed when it stored: the length-prefixed UTF-8 of, in this order, kind, learned, term, the chosen end, capture, fact, task, run, the number of tags and then the tags folded and sorted, ref, said-by, asked-by, recorder, source, session, turn, observed time in UTC, and the SHA-256 of the text; absent is distinct from empty. It leaves out the received time, the recording checkout and the keywords, which pbrain derives, so a retry from another worktree after a lost reply is the same request. One teaching event has one writer (spec 6.4), so the recorder is part of it.

```
admit(event, payload):
  if observed > now + 5 min or observed < now - 14 days:  refuse (invalid)      # before any lookup
  r = receipt[event]
  if r == nil:                         admit; add the receipt
  elif r.digest != digest(payload):    conflict
  elif r.gone:                         "no longer available"; write nothing
  else:                                return r.mem_id, duplicate = true        # before any quota check
```

A receipt is deleted when its `observed` is more than 14 days old; from then on the first line refuses any replay of it. So no path recreates a forgotten memory. An unchanged replay is answered before the quota is looked at, so it succeeds against a full allocation.

## 5. Memory: the operations

### 5.1 The interface

```go
package memory

// Root is one physical section's memory, open for one command.
type Root struct{ /* db *sql.DB; dir sys.Dir; files sys.Files; now func() time.Time; rand io.Reader */ }

func Open(ctx context.Context, files sys.Files, dir sys.Dir, opt Options) (*Root, error) // dir: the enrolment's folder
func (r *Root) Close() error

type StoreRequest struct {
    Row  Row     // every caller-supplied and observed field
    Text []byte  // nil for a report
}
type StoreResult struct{ Row Row; Duplicate bool }

func (r *Root) Store(ctx context.Context, req StoreRequest) (StoreResult, error)
func (r *Root) Recall(ctx context.Context, q Query) (Page, error)
func (r *Root) Text(ctx context.Context, id string) (Row, []byte, error)
func (r *Root) List(ctx context.Context, q ListQuery) (ListPage, error)                       // memory log, memory highlights
func (r *Root) Sessions(ctx context.Context, since, until time.Time, limit int, after Cursor) (SessionsPage, error)
func (r *Root) Forget(ctx context.Context, id string) (Row, error)
func (r *Root) Preview(ctx context.Context, p PreviewRequest) (Preview, error)
func (r *Root) Show(ctx context.Context, token string) (Preview, []byte, error)
func (r *Root) Confirm(ctx context.Context, token string, ids []string, publish PublishFunc) (Confirmed, error)

// PublishFunc publishes the frozen draft and reports whether its commit point was reached.
type PublishFunc func(ctx context.Context, target Target, draft []byte) (published bool, err error)

type ListQuery struct {
    Since, Until time.Time   // on received; zero is open
    Labels       []string    // each must be carried, as a tag or as a keyword
    Actor        string      // recorded_by or said_by
    Task, Run    string
    Session      string      // every row of that session
    Bucket       Bucket      // exactly one row of Sessions; excludes Run and Session
    Kinds        []Kind
    Term         Term
    Highlights   bool        // the selection and the grouping of spec 5.6
    Limit        int         // 1 to 200
    After        Cursor      // zero: from the newest
}
type Bucket struct {                 // zero: no bucket asked for
    Kind    uint8                    // run: every row with Run; session: rows with Session and no run; none: rows with neither
    Run     string
    Session string
}
type Cursor struct {                 // what the first page fixed, and the position of the last row returned
    Group        uint8
    Received     int64
    ID           string
    Since, Until int64               // the instants the first page's window resolved to; zero is open
    High         int64               // the greatest rowid at the first page: later admissions are above it
    Bind         [16]byte            // a digest of the enrolment id, the kind of list and the normalized query
}
type ListPage struct {
    Rows    []Row            // each with Run, Tags, Keywords, the first 160 characters of its text, and Group when Highlights
    Next    Cursor
    More    bool
    Dropped DropCount        // by reason, for the period asked for
    Counts  GroupCounts      // Highlights only: the three groups and every memory of the period
}
type SessionsPage struct { Rows []SessionRow; Next Cursor; More bool }
type SessionRow struct {
    Bucket       Bucket      // its exact selector
    First, Last  time.Time
    Memories, Learned, Blockers, LongTerm int
    Agents, Tasks []string   // at most five each
}
```

`cli` owns everything that needs the brain: which section, the folder handle, the author, and the checkout's branch and commit (two read-only Git queries added to `gitx`: `Head` and `Dirty`). `memory` runs the secret scan itself, on the complete row, so no caller can forget it.

### 5.2 The order of a store

The order is fixed, because each step protects the next.

1. Parse the flags. Every usage error of spec 5.1 is decided here, before standard input or any source is read.
2. Open the brain with its normal guard; choose the section; resolve the author.
3. Get the text: the argument, standard input read to at most `TextMax + 1` bytes, or the host reader (section 7). More than the limit is `too_large`.
4. **Assemble the complete row**: the caller's fields, with the run from `--run` or else from `PBRAIN_RUN` (a new entry in the audit's list of environment reads, beside `PBRAIN_TASK`), then everything observed: the received time, the recording branch, commit and dirty flag, and what the session record and the host reader supply. For a store by a supervisor into the shared section, only the enrolled project name and the branch and actor details the supervisor is permitted to give are kept; a raw path is never part of a row.
5. **Validate and scan the whole row**: UTF-8, not empty, not control characters only, each field's length, `MetaMax`, the task, run, tag and source forms, with the tags folded and repeated ones removed; then the secret scan of the text **and of every metadata value, observed ones included**, since a branch name or a session-derived value can carry a secret as well as a caller's text can. A match is `secret_refused` naming the field. Nothing has been written or logged up to here.
6. Open the memory folder of the section's place (section 6.3). No enrolment, or a replaced project: `memory_not_enabled`.
7. Check the database files, open the connection and check its identity (sections 6.5 and 4.4).
8. `BEGIN IMMEDIATE`. Busy past the limit: `memory_busy`.
9. Delete expired rows, which takes their words with them, and mark their receipts gone; delete receipts past their horizon.
10. The replay check (4.7); then the allocation, the caps and the byte budgets (4.5), counting each row a count limit removes in `dropped`, and deleting `dropped` rows older than 14 days.
11. Derive the keywords (4.3), unless the row is a report. Insert the row, its distinct words, its tags and keywords as `label` rows, and the receipt if keyed. `COMMIT`. The commit is the one commit point.
12. Close; print the receipt.

### 5.3 Recall

One read transaction, so the whole answer comes from one committed state.

1. **Candidates.** With words: the union of the `word` rows for the query's distinct words, grouped by memory, with the count of distinct query words each memory holds. The spec admits a memory that holds at least one. Without words: the rows selected by task, session, kind or term.
2. **Eligibility**, in the same statement: the section's database, not expired, the task, session, run, kind and term filters, and for each label asked for one `EXISTS` look-up in `label` by its primary key. Labels only narrow; they are not in the order. A label on a read is validated apart from a tag on a store: `cli` folds it with the scanner's fold and accepts a value that is a tag or one word the keyword rule could yield (3 to 32 characters, any script), from the command line, the catalog and the screen alike; anything else is `usage`.
3. **Order**, on the whole candidate set and before any limit or offset: authored before reports; then the count of distinct query words; then a full phrase match, which is decided by reading the candidate's text with `query`'s matcher, the one search uses; then kind, newest `observed`, id. A query of one word needs no text for this step, since the phrase test is the same for all its candidates. No `LIMIT` is applied in SQL before the phrase test, because a later candidate can win on it.
4. **Display**: only the rows of the page have 400 characters of their text read for showing.

The full scan of every text, by the same matcher, is kept in the tests as the oracle: for every query of the parity set, both paths must return the same ids in the same order. The set includes folded Unicode, a word repeated in the query, a phrase that only a late candidate holds, a word in every row, and a typed report.

`--banked` is composed in `cli`: it runs the recall, then the existing search entry point (pbrain architecture 8.5) or the document search of section 10, by words only, with half of `--max-chars` reserved for it, and hands `render` two labelled groups. `memory` knows nothing of items.

The task page is a `Query` with a task, no words, limit 3, on the shared section, and a renderer mode that fixes the layout and the 1,200-character budget. The hook and the command call the same function.

### 5.4 The lists a person reviews with

`memory log`, `memory highlights` and `memory sessions` (spec 5.5, 5.6) are reads like recall: one read transaction each, nothing written, the same eligibility (this section's database, not expired).

- **The log** is `List` with `Highlights` false. The order is `received` descending, then `id` descending. The index `memory_recv` serves it; with a run or a session, `memory_run` or `memory_session` does. The other filters (labels, actor, task, kinds, term, a bucket) are conditions on the rows that walk yields.
- **A list is fixed by its first page.** Position alone does not keep later arrivals out: a row received in the same second with a lower id sorts below the cursor, a clock set back stamps a new row among old ones, and in highlights a new row of a later group sorts below any position in an earlier one. So the first page records, in the same read transaction, `High`, the greatest `rowid` in the table. `rowid` is declared `AUTOINCREMENT`, so it rises with every admission in this enrolment and a deleted value is never given out again, which a plain integer key does not promise. Every later page adds two conditions to its statement: `rowid <= High`, and strictly below the cursor's `(group, received, id)` in the list's order. It still applies expiry and reads the table as it is, so a forgotten or expired row is gone from later pages. No reader stays open between pages: the cursor is all the state there is.
- **A cursor is bound to its list.** `Bind` is a digest of the enrolment id, which says the section and the database, the kind of list (log, highlights, sessions), and the query with its filters in a normal form: labels and kinds sorted, the window left out because the cursor carries it. A page asked for with a cursor whose `Bind` does not match the query it comes with is `usage`; so is a cursor from before the memory was enabled again.
- **Highlights** is `List` with `Highlights` true. First the exclusions: only rows whose capture is authored or verbatim and whose kind is not handoff. Then each remaining row is given the first group it fits (1 learned; 2 kind blocker; 3 any other long-term row), and a row that fits none is left out. So a long-term handoff is never in group 3, and a long-term outcome is. The order is the group, then `received` descending, then `id` descending, and the cursor carries the group. `Counts` is one aggregate statement over the same period in the same transaction.
- **Sessions** is one grouped statement over the rows received in the period, keyed by the run where there is one, else by the session, else by neither, with the counts of spec 5.6; the agents and tasks of a row are cut to five in Go. Each row carries its `Bucket`, and `List` with that bucket applies the very predicate that formed it (`run = R`; `session = S AND run IS NULL`; `run IS NULL AND session IS NULL`), so a row's count and its list agree. The bucket is a typed value, not a reserved name: a session called `unassigned` or `none` is a session. The order is the last arrival descending, then the bucket's kind and key, and it is paged by that position with the same `High` and `Bind` as a log, so every bucket is reached however many share a second. `High` keeps later admissions out, but it cannot freeze an aggregate: when a bucket's latest row is forgotten, expires or is dropped between two pages, its last arrival becomes an earlier one. It can only move that way, so a bucket already shown can come again below the cursor and a bucket not yet shown stays below it: none is missed, and one can repeat (@architect-2's case: A at 30 and 10, B at 20, a page of one, the row at 30 forgotten, then B and A again). The spec says so (5.6), and the screen keys its rows by selector and replaces the earlier one. `cli` writes a selector as `run:<run>`, `session:<id>` or `none`.
- **The period** of highlights with neither run, session nor time is the last 12 hours, set in `cli` from the clock it passes down; `memory` has no default of its own. A span is resolved to instants once, for the first page, and returned in the cursor; a page asked for with a cursor takes its window from the cursor, and `cli` refuses `--since` or `--until` beside `--after`. A list therefore does not slide while it is read.
- **`Dropped`** is the sum of `dropped.n` by reason over the whole hours the period touches, among the hours of the last 14 days by the clock of the read: with no period, over all of those. It does not depend on the other filters or on whether any live row matches. `cli` prints `memory_dropped` when it is not zero, and the message says that it counts whole hours of arrival within 14 days.
- **The cursor** is rendered by `cli` as one opaque token: the whole `Cursor`, with its position, its window, `High` and `Bind`, and a short checksum. A token that does not parse is `usage`.
- **The row on screen**: 160 characters of the text, read only for the rows of the page, put on one line by `render.OneLine` after `render.Inert`. `render.Inert` is new and belongs to phase 1, because these lists need it: it removes from a string every C0 control except newline and tab, DEL, every C1 control, and every escape sequence. Its companion `render.Visible` removes nothing: it writes each of those characters, and a backslash, as a visible escape (`\x1b`, `\\`), so that its output is plain text from which the original can be read back exactly. The screen's exact view uses it (5.5).
- `memory show` is the function behind `recall --id`, registered under a second command name and not as a second tool.
- In the tool catalog: `memory log`, `memory sessions` and `memory highlights`, read-only. Not `memory show` and not `memory browse`.

### 5.5 The screen

`memory browse` (spec 5.7) is the package `browse`: a Bubble Tea program whose model holds a list, a detail view and a filter, built from the components of `bubbles`, styled with `lipgloss`, with the expanded text formatted by `glamour`.

```go
package browse

// Source is every read the screen makes. cli implements it by opening the memory, reading in one
// transaction and closing it again, for each call.
type Source interface {
    List(ctx context.Context, q memory.ListQuery) (memory.ListPage, error)
    Sessions(ctx context.Context, since, until time.Time, limit int, after memory.Cursor) (memory.SessionsPage, error)
    Text(ctx context.Context, id string) (memory.Row, []byte, error)
}

// Actions is every change the screen can ask for. cli implements each with the function behind the
// command of the same name, so the checks, the turns and the confirmation token are the commands' own.
type Actions interface {
    Forget(ctx context.Context, id string) error
    Preview(ctx context.Context, p PromoteRequest) (token string, err error)                  // records the token, as promote does
    Show(ctx context.Context, token string, offset int) (page string, more bool, err error)   // promote --show: the frozen draft, a bounded page at a time
    Confirm(ctx context.Context, token string, ids []string) (written string, err error)
}

type PromoteRequest struct {
    IDs     []string
    Kind    string // "learning" | "decision"
    Title   string
    Decided string // who decided; required for a decision
}

type Options struct {
    Query      memory.ListQuery // the filters given on the command line
    Highlights bool
    In         io.Reader        // a terminal
    Out        io.Writer        // a terminal
    Now        func() time.Time
}

func Run(ctx context.Context, src Source, act Actions, opt Options) error
```

How it keeps the promises of spec 5.7, by their numbers there:

1. **One section.** `cli` builds `Source` and `Actions` for the section the command was started with and gives `browse` nothing else: no brain, no other root, no flag to change it. There is no code path in `browse` that could reach the other section.
2. **Inert text.** In the exact view the text goes through `render.Visible` (5.4) and is drawn as plain text, with no Markdown rendering. Everywhere else `render.Inert` (5.4) is applied to every value that comes from a memory, text and metadata alike, before it is given to `lipgloss` or `glamour`; the commands' lists use it too. The output of `glamour` then passes a second filter that lets through its colour and style sequences (CSI … `m`) and nothing else, so that no link or image escape reaches the terminal whatever the Markdown says and whatever a later version of the library does. The style is one fixed style compiled into pbrain; none is read from the environment or asked of the terminal.
3. **Read again before acting.** Forget and promote take ids, and the functions behind them read the rows afresh; `not_found` is shown as "gone" and the list is refreshed.
4. **Confirmation.** Forget shows a confirmation in the screen and then calls `Actions.Forget`. Promote calls `Preview`, reads the frozen draft back through `Show`, the checked path of `promote --show`, a page at a time, and calls `Confirm` with the token only on the person's key; there is no other path to `Confirm`. These are the grouped-promotion functions of phase 2, which the screen therefore needs.
5. **Nothing held.** `Source` and `Actions` open and close the database inside each call, as a command does. Between two keys the process holds no connection, no transaction and no turn.
6. **Nothing written by looking.** `browse` opens no file and has no configuration; Bubble Tea runs on the alternate screen, so the texts are not left in the terminal's scrollback. It imports neither `embed`, `semantic` nor `fetch`. `Source` has no method that writes. What is written comes only through `Actions`, at the person's request: `Preview` records its bounded token row, which lapses after ten minutes if it is not confirmed, as any preview's does; `Forget` and `Confirm` have the effects of their commands.
7. **Looking changes nothing.** No call of `Source` changes a row.
8. **A terminal only.** `cli` checks that standard input and output are terminals before anything else and fails with `usage` otherwise; the command has no `--json` and no catalog entry.

**The dependencies**, chosen by the owner: `charm.land/bubbletea/v2`, `charm.land/bubbles/v2`, `charm.land/lipgloss/v2` and `charm.land/glamour/v2`, imported by `browse` alone; `buildcheck`'s import rule gains a row that says so. What @lead-dev established on 2026-11-27, with a scratch program linked into pbrain at `53d0ebf` (pins `v2.2.0`, `v2.2.2`, `v2.1.1` and `v2.0.1`; Linux amd64 run natively, the other three cross-built):

| Question | Finding |
| --- | --- |
| No cgo on linux/amd64, linux/arm64, darwin/arm64, darwin/amd64 | Yes, in the plain and in the accelerator builds |
| Size | About 10 to 12 MiB more in each stripped release binary |
| Licences | The four are MIT; the modules they bring are 24 MIT and 4 BSD-3-Clause, none with a separate notice file. Their texts go into `licenses/` for the final pinned set, as #79 does |
| Output of five commands at start | On the natively run Linux build, `--help`, `--version`, `schema`, `schema --format mcp` and `skill` print the same bytes with the libraries linked. These are samples of start-up output, not the audit |
| The release audit of #55 (`TestRuntimeEnvAndOutput`) | **Fails, 7 of 7**, where the build without the libraries passes 7 of 7. Go runs a library's start-up code in every command, and choosing the command late does not defer it. At start, `x/ansi` and `go-runewidth` read `RUNEWIDTH_EASTASIAN` and the locale variables; on Linux the list component of `bubbles` brings in clipboard code that reads `WAYLAND_DISPLAY` and searches `PATH`. Without that component the audit still reports 90 sites on Linux amd64 |
| Can pbrain fix the width rule in code, so that the variable has no effect? | **No, at these pins.** `x/ansi` keeps its width settings private, and Lip Gloss and Glamour use those: with the environment cleared and the exported setting of `go-runewidth` reset, the three still disagree |
| Files written at a variable's bidding | When a program is created Bubble Tea honours `TEA_TRACE` and can open the file it names; on a panic, with `TEA_DEBUG` set, it prints the panic value and can create a `bubbletea-panic-*.log` file. Giving it explicit streams and settings does not turn either off |

Three things follow in the design whatever is decided next:

- `browse` does not use the list component of `bubbles`. It draws its own list and uses the viewport, which takes the clipboard code out of the binary.
- The screen must not be able to write a trace or a crash file, or print a panic's value, whatever variables are set. Tests force a failure inside the screen, in the event loop and in a background task, with a made-up marker in a memory and those variables set, and find the terminal restored, no marker in the output and no file.
- The Markdown style is fixed in pbrain (above), and the terminal's colour profile is given to the libraries explicitly.

**What is left is a decision for the owner, and phase 6 waits for it; nothing else does.** The audit's rule is not changed in either option: pbrain reads no environment variable that is not on its list and writes nothing at a variable's bidding. In option B the audit's test has to learn about a second executable; the rule it enforces is the same.

- **Option A (@lead-dev's recommendation): the screen inside pbrain, after the libraries are changed.** The start-up reads are removed and the trace and panic paths are turned off, by changes accepted upstream or by small patches that pbrain pins; then every remaining site is reviewed and the unchanged audit is run on every release build. Its cost: waiting on other projects, or carrying patches at every update. How a patch is carried matters: a `replace` line in `go.mod` makes `go install` of the module fail, which the README offers today, so the patches would have to live in forks under their own module paths.
- **Option B (@architect's recommendation): the screen as a second executable**, `pbrain-browse`, built from the same repository, and started by `pbrain memory browse`. pbrain itself links none of the four libraries: its size and its start are as today, with no patch and no fork. @lead-dev checked the facts this rests on and corrected two of them; what follows is with those corrections.
  - **The packages.** The four libraries stay out of everything that `cmd/pbrain` imports. The code that builds `Source` and `Actions` for one section lives in a package without them, and `cmd/pbrain-browse` imports that package and `browse`.
  - **The launch.** pbrain resolves the helper as the file beside its own executable, by absolute path, and never through `PATH`: the installation folder is what is trusted. It starts it with an environment it builds itself and passes explicitly, which takes effect before any library's start-up code runs (@lead-dev's scratch launcher showed the child's widths fixed whatever the parent's variables were). That environment is a short named list: what the terminal needs, a fixed width rule, and the few of pbrain's own variables the screen needs. It is not a copy of the parent's, and it holds none of the libraries' debugging variables.
  - **The same build.** The helper refuses to run unless its build is the same as pbrain's. That is a check of compatibility, not of trust. A helper that is missing or of another build gives a message that says how to install it and names `memory log`.
  - **A failure inside the screen.** One recover around the program is not enough, because Bubble Tea's renderer and its tasks run in goroutines of their own. So the library's own panic handling is turned off; the helper's error output goes to pbrain over a channel pbrain controls and is never copied to the terminal; and pbrain, the parent, restores the terminal and prints one fixed line when the helper dies. Tests force a panic in the event loop and in a background task, each with a marker in a memory, and find the terminal restored, no marker and no file.
  - **The audit.** Its rule stays. Its test does not stay as it is: `TestRuntimeEnvAndOutput` today expects every released executable to link the inference library with its accelerator set, and reports any new start of a subprocess. So it gains an expected dependency set for the second executable, one named and reviewed launch site in pbrain, and a review of the helper's own start-up and dormant sites against the environment pbrain gives it.
  - **Its cost:** a second file in every release archive, the release matrix, archive and licence tests for the pair, and a second thing for a `go install` user to install. Option A is the simpler install once the changes it waits for exist.
- **Not available today: listing the start-up reads in the audit**, as the database driver's `MODERNC_SQLITE_OFD_LOCK` is listed. That entry is sound because pbrain fixes the driver's setting in code; here the setting cannot be fixed (table above), so the reads would change what is drawn. It becomes possible only with a change in the libraries.

In both options `browse`, its two interfaces and its tests are as described here; they differ only in which executable links them.

**Tests.** The model is tested without a terminal: key and data messages go in, the view comes out as text. One run through a pseudo-terminal on Linux and on macOS proves start, a key and a clean exit. The cases are spec scenario 54.

## 6. The two roots, keys, locks and the one kind of file opened by path

### 6.1 The roots

`guarded.Open(ctx, files, opt) (*Root, error)` opens a per-user root, the cache or the state folder: resolve it once, check the owner and the mode, and hand out children only as single-component `sys.Dir` opens. `semantic.OpenCache` is rewritten to call it.

The placement rule is new and stricter than today's (spec 12.2). `guarded.Roots(cache, state)` resolves the cache and the state root and fails with `invalid_config`, naming the variable, unless they are two different folders, neither inside the other, and both outside every Git work tree, the brain and the personal section. Today `OpenCache` accepts a cache inside a work tree when Git ignores it; that case is now refused, nothing is written there, and the release note says so. The check runs before anything is created, moved or deleted, and only in commands that need a root: `search`, `get`, `list`, `prime` and `check` without `--semantic` never call it.

`store.OpenInput` gains one rule beside its `.pbrain` rule: an input whose resolved path lies in either root is refused, as a protected path is. Everything new that pbrain keeps in them is also below a `.pbrain` folder there, the records database included, which holds a copy of a host's settings file as it was before `hooks install`: so it is refused as an input by the standing rule as well, whatever the roots are set to.

### 6.2 Layout

```
<cache>/models/, <cache>/vectors/                      as today; nothing moves
<cache>/.pbrain/records.db                             the cache's records: the ids its folders are named by (6.3)
<cache>/.pbrain/index/v1/<project-key>/<repo-id>/<checkout-id>/items/index.db   the shared item index
<cache>/.pbrain/index/v1/<project-key>/<repo-id>/<checkout-id>/docs/index.db    the document index
<cache>/.pbrain/index/v1/personal/<personal-id>/items/index.db                  the personal item index
<cache>/.pbrain/vectors-personal/v1/<personal-id>/     personal vectors, never read by a shared build

<state>/.pbrain/records.db                             the state's records: ids, enrolments, moves, installed hooks (6.3)
<state>/.pbrain/turns/v1/<kind>/<turn-key>.db          the turn databases: nothing in them but the turn (6.4)
<state>/.pbrain/memory/v1/<project-key>/<enrolment-id>/memory.db          shared memory
<state>/.pbrain/memory/v1/personal/<personal-id>/<enrolment-id>/memory.db personal memory
<state>/.pbrain/originals/v1/<project-key>/<repo-id>/<checkout-id>/<sha256>    originals of shared imports
<state>/.pbrain/originals/v1/personal/<personal-id>/<sha256>                   originals of personal imports
<state>/.pbrain/sessions/<session-id>.json             host sessions (7.2)
```

There is one index database per index folder, one memory database per enrolment, one records database per root, and no lock, marker or receipt file anywhere. Each database has SQLite's own side files beside it while it is in use.

### 6.3 Keys, identities, and a reused path

- **Project key:** the lowercase SHA-256 of `git-v1`, NUL, the absolute Git common directory, NUL, the brain's path from its work tree root with forward slashes. Outside Git: `dir-v1`, NUL, the resolved brain path. `gitx.CommonDir` is a new read-only query. Two brains in one checkout have two project keys; every path of 6.2 that is "per project" contains the key, and every database records it (6.5), so one brain's database can never pass for the other's.
- **A place** is a physical folder whose replacement must be noticed. There are three kinds, and each kind of data hangs on the one that matches how it is shared:

| Data | Logical key | Physical place it is bound to |
| --- | --- | --- |
| Shared memory | the project key | the repository: its Git common directory; outside Git, the brain folder |
| Shared indexes, originals and the receipt of a move | the project key | the repository **and** the checkout: the common directory and the work tree root |
| Personal memory, indexes, vectors and originals | none | the personal section's root |

  A checkout folder can stay while the repository inside it is replaced (a new `.git` in the same folder): the path, the project key and the checkout's identity are then all unchanged. So everything shared is also bound to the repository's own incarnation, the `<repo-id>` in its path: a replaced repository gets a new one by itself, starts with no originals, no index and no finished move, and its first `import` and `sync` work with no setup. Shared memory is bound to the common directory and not to the brain folder, because `docs/brain` in worktree A and in worktree B are two folders of one brain: `memory init` in A, a store in B and a recall in A must meet in one database. Personal data carries no project key, because two clones that a person points at one personal folder must both be able to use it; default personal sections are different folders and stay apart.
- A place's record is a row of the `place` table in that root's records database: the kind, the resolved path, the identity of the folder and a random id. An enrolment is a row of `enrolment` in the state's records database: for shared memory keyed by the project key, so two brains of one repository have two enrolments on the same common directory; for personal memory keyed by the personal id.
- **Identity** is the device, the inode **and the birth time** of the folder. Device and inode alone can come back after a folder is deleted and another is created; the birth time does not. Linux (`statx`) and macOS report it on the local file systems pbrain supports. Where a file system reports none, continuity cannot be established: a command that needs a bound place fails with `invalid_config`, naming the folder, and commands that only read items are not affected.

```go
package guarded

type Identity struct {
    Device, Inode uint64
    Born          time.Time // must be non-zero
}

type Place struct {
    Kind     string   // "repository" | "checkout" | "personal"
    Path     string   // resolved
    Identity Identity
    ID       string   // 128 random bits, hex: a <repo-id>, <checkout-id> or <personal-id>
}

// Bind returns the place for dir. The read, the comparison and the write are one BEGIN IMMEDIATE
// transaction on the root's records database, so two callers cannot both rebind. With no record it creates one.
// With a record of another identity the folder was replaced: with rebind false it returns ErrReplaced;
// with rebind true it replaces the row with a new id in that same transaction and returns that.
func (r *Root) Bind(ctx context.Context, kind string, dir sys.Dir, rebind bool) (Place, error)
```

- **Each root binds for itself.** The cache has its own records database, for the folders of the indexes; the state has its own, for originals and memory. A purged cache simply gets new ids and new indexes; the state's ids stay. The two ids of one checkout are unrelated.
- **Readers never bind.** `Lookup(ctx, kind, dir) (Place, bool, error)` returns the existing place, or "none", or `ErrReplaced`, and creates and changes nothing. A search and `sync --status` use it: no place or a replaced one is simply "no index". Only a writer binds: `sync` and `docs sync` in the cache; `import`, the move and `memory init` in the state. A plain `add` binds nothing: its turn is keyed by the folder itself (6.4).
- **A repository, a checkout and a personal section bind and rebind by themselves** when a writer needs them, so a new brain's reads, imports and `sync` work with no setup. A worktree created again at an old path, or a personal folder replaced by another, gets a new id and so an empty index and no originals; the old id's folders are left on disk and never opened again.
- **Memory is enabled only by `pbrain memory init`**, for the shared section, and by `memory init --personal` for the personal one, each with its own enrolment row and its own enrolment folder. Without init, memory is `memory_not_enabled` and everything else works.
  - Init is idempotent: where an enrolment row exists and its identity matches, it changes nothing and prints the same identity, and it never resets a memory that is in use.
  - Init never adopts a file it finds. It makes a **new** folder named by a fresh random enrolment id, creates `memory.db` in it, sets WAL, commits the schema and the identity rows, syncs, and only then, in one transaction on the records database, inserts the enrolment row (or replaces it, when the bound folder was replaced). That transaction is the commit point. An init that is killed before it leaves a folder that no row names, which nothing ever opens. Two inits at once each make a folder; the transaction lets one in, the other finds the row and uses it, and its own folder is left unnamed.
  - **What an enrolment is bound to depends on which memory it is.** The shared one holds the project key and the identity of the repository's common directory; it is replaced only when that repository was replaced. The personal one holds the identity of the personal section's folder and nothing about any project or repository; it is replaced only when that folder was replaced. So `memory init --personal` from a second clone that uses the same personal folder finds a matching row, changes nothing, and both clones read the same personal memory.

Two clones have two common directories and so two project keys. Two clones that a person points at one personal folder resolve the same personal place and share its index, vectors, originals and memory, as they share its items.

### 6.4 Taking turns

pbrain keeps no lock files (the owner's decision of 2026-11-27). The item writers' lock was a file in `<section>/.pbrain/`; the index and the model download each had a lock file. All of them become **turns**: an open SQLite write transaction on a small database in the state root that holds nothing else.

```go
package guarded

// Turn waits for the turn of key and returns its holder. The wait ends at ctx's deadline or after
// limit, whichever is first, with ErrBusy and nothing changed. Release ends the turn; it is safe to
// call after ctx was cancelled, and the turn is not given up before Release is called, so file work
// already under way finishes inside it. A process that dies gives the turn up at once.
func (r *Root) Turn(ctx context.Context, kind string, key TurnKey, limit time.Duration) (*Held, error)
func (r *Root) TryTurn(kind string, key TurnKey) (*Held, error)            // one attempt, for catch-up
func (r *Root) ReadTurn(ctx context.Context, kind string, key TurnKey) (*Held, error) // shared, for index readers
func (h *Held) Release() error
```

**How a turn works.** The database is `turns/v1/<kind>/<turn-key>.db`. It is created empty with one table that is never written. `Turn` opens one connection, pins it, and runs `BEGIN IMMEDIATE`; SQLite admits one such transaction at a time. `Release` runs `ROLLBACK` and closes. Nothing is ever committed to a turn database, so there is no "held" row that a crash could leave behind, and no journal file is ever created. The wait is SQLite's busy timeout in short steps inside a loop that watches the deadline.

Turn databases are in **rollback-journal mode**, not WAL, for two reasons: no side files are needed to open one, so a reader can hold one on a folder it cannot write; and in this mode SQLite gives exactly three levels, which the index uses: a read transaction that has read something is *shared*, `BEGIN IMMEDIATE` is *reserved* and lets readers in, and `BEGIN EXCLUSIVE` waits until the readers have gone. A WAL database's write transaction never waits for readers, so it could not do the third. The data databases (memory, indexes, records) stay in WAL.

**The key is the thing itself.** `<turn-key>` is the SHA-256 of the kind and the identity (device, inode, birth time) of the folder being protected, with a file name added where it is one file. It is a pure function of the physical target: no record is needed, a plain `add` binds nothing, and every process that uses this state root computes the same name.

| Turn | Kind | Key made from | Held |
| --- | --- | --- | --- |
| Writing a section's Markdown | `section` | the brain folder of this checkout; or the personal section's folder | `Turn`, 30 s or the caller's limit, from before the scan to after the last file operation |
| Building an index | `index` | the index's folder in the cache | `Turn` by `sync`, waiting as today after its progress notice; `TryTurn` by a search's catch-up; `ReadTurn` by every reader of that index, from before it opens the index database until after it closes it |
| Downloading and verifying a model | `model` | the cache's models folder, with the model's name | `Turn`, as the per-model lock of today |

- `store.Begin` takes the section's turn before it scans and keeps it until the write and its clean-up are done. A command that cannot open or write the state root fails with `io`, naming it, before any item is touched. A shared write takes only the shared section's turn and opens nothing personal.
- Two worktrees have two brain folders, so two turns. Every checkout or clone that reaches one personal folder computes the same key and waits on it.
- A reader of items takes no turn: `search`, `get` and `list` without `--semantic` need no state root.
- **Everything that opens an index holds its read turn**, from before it opens the database until after it closes it: a semantic search, `sync --status`, a build's own first read. A read turn needs only read access to the turn database, which a build created; where the state root cannot be read, or no turn database exists yet, the index is not opened: a search answers by words with `semantic_not_ready`, and `sync --status` reports that index as not readable. Taking a build turn, or catching up, needs the state root writable.
- Embedding and downloading never happen inside a section's turn or a memory transaction: they have their own.
- **What does not need a turn:** ordinary memory writes (the memory database's own transaction), index row updates (the index database's own), binding and enrolment (one transaction on the records database), and `hooks install` (two short transactions on the records database, 8.3).

**One rule, stated in the spec (12.5).** Processes take turns only on the same database, so everything that works on one target, be it a checkout, a personal section, an index or a model cache shared between projects, resolves the same state root. Two processes with different state roots on one target are not supported and cannot be detected. A turn also cannot stop a person or Git from changing the files, as no lock could.

### 6.5 Opening a database by path, and how that is contained

pbrain reaches every file it owns through an open folder handle and never through a link (pbrain architecture 10.3). SQLite cannot: it opens a database and its side files by path name, and the driver does not ask it to refuse links. This is the one exception. It covers pbrain's own databases in the two per-user roots, which are the memory, index, records and turn databases, and nothing else: never a path in a repository, never a path a caller supplied.

**Who could abuse it.** A process of the same user that may already write that folder or an ancestor it can replace. Another user cannot: the folders are owner-only. It matters where the pbrain that follows a redirected path has wider file access than the process that redirected it.

**What pbrain does, in order** (`guarded.OpenDB`, used by `memory` and `semantic`):

1. The folder is created by pbrain with mode 0700 inside a root that `guarded.Roots` accepted. The database's path is built by pbrain from the root and the ids of 6.3.
2. Through the folder handle, **without opening the files**: the database and its `-wal`, `-shm` and `-journal` names are each absent, or a regular file of this user, mode 0600, with one name and not a link. Otherwise `io`; nothing is opened.
3. **A database is created only on the paths meant to create one, and always by publishing a finished file.** `memory init` creates a memory, in a fresh enrolment folder that no row names yet (6.3); `sync` and `docs sync` create an index, during the index's turn; a writer creates a records database or a turn database on first use. In each case the creator builds the database under a private temporary name in the same folder: schema, identity rows and journal mode, then a checkpoint and a clean close so that every byte is in the one main file, then a sync. Only then is the closed file given its real name by an atomic rename that fails if the name exists (`Dir.RenameNew`, a new operation on the repository's existing no-replace rename for Linux and macOS). A rename, not a second link: a link would leave the file with two names, which item 2 refuses, and a creator killed between linking and unlinking would leave it refused for good. Then the folder, and every parent folder that was newly made, is synced, before the creation is acknowledged or anything refers to the new database. Of two creators at once one wins; the other discards its own file, syncs the folder itself in case the winner had not yet, and opens the winner's. So no process ever sees a half-made database under a real name, and a finished one always has exactly one name. A creator that finds side files at the real name with no main file refuses with `io` for a memory, a records or a turn database, which never have a reason to be in that state; for an index it goes through the rebuild of item 7. `store`, `recall`, `forget`, `promote` and every search never create a memory or an index: a missing memory database is `memory_not_enabled`, a missing index is today's "no index".
4. The connection is opened with a fixed, encoded file URI. On an existing database no statement that could change its journal mode is issued; the settings of 4.4 that are per connection are set and read back, and `journal_mode` is only read and must be the one that kind of database has: `wal` for memory, index and records databases, `delete` for turn databases.
5. **Identity before use.** Each database has a `meta` table with a tagged identity, by the table of 6.3: shared memory has the project key and the enrolment id; a shared index has the project key, the repository id, the checkout id, the corpus (items or documents) and the profile rows of 10.4; personal memory and a personal index have the personal id, the corpus where it applies, and no project key. pbrain reads those rows and requires its own values before it reads any other row into a result or writes anything. A database of another project, checkout or section is closed and the command fails with `io`. SQLite itself may have read the header and finished an interrupted commit before that; the check gates pbrain's use of the data, not SQLite's open.
6. While a connection is open, pbrain never opens and closes the database or a side file through another descriptor; with the portable lock mode, closing any descriptor of a file drops this process's locks on it. Checks during that time use metadata calls on the folder handle.
7. **What happens when a database is not sound depends on what it holds.** An open that fails because the database is busy, or because of permissions or an I/O error, is reported as that and is never taken for damage.

| Database | If it is damaged, empty, of the wrong mode, or another place's |
| --- | --- |
| Memory | Never removed, repaired or overwritten. `io`, naming the folder; the person decides |
| The state's records | The same. It says which memory and which originals belong to which project, and holds what `hooks install` saved; without it intact memories would be unreachable |
| A turn database | The same. Replacing one could let two writers in at once, each on its own file |
| The cache's records | `io`, with the advice to clear the cache with nothing running; nothing in the cache is irreplaceable |
| An index that is **another place's** but sound | Left alone; `io` |
| An index that is **this place's**, or cannot be read at all, and is damaged (SQLite reports corruption or "not a database", or `quick_check` fails) | Rebuilt by `sync`, as below |

   Rebuilding is the only case in which pbrain removes a database file. The order is fixed, because the `sync` that finds the damage is itself a reader of the index at that moment and would otherwise wait for itself:

   1. `sync` closes its own connection to the index, then releases its own read turn if it holds one, then its build turn. From here it holds nothing on the index.
   2. It takes the index's turn **exclusively**: `BEGIN EXCLUSIVE` on the turn database, which waits until every reader's `ReadTurn` and every build turn has ended, and lets no new reader or builder in while it is held.
   3. It checks again, through a fresh open that it closes before going on, that this is the index of this place and that the file is still the damaged one. If another `sync` rebuilt it in the meantime, nothing is removed and that replacement is used: go to step 6.
   4. It removes the database, then its side files, and syncs the folder.
   5. It publishes a new, empty index by the rule of item 3, which syncs the folder. The exclusive turn is held from step 2 to here.
   6. It releases the exclusive turn, takes a build turn as any `sync` does, opens the new index and builds. An index folder that holds side files and no main file is a rebuild that was killed between those removals: `sync` treats it as damaged, so it takes the exclusive turn, removes the leftover side files, syncs the folder and only then publishes. A new main file is therefore never opened beside an old database's side files. If the readers do not go within its limit it leaves the file and tries at the next `sync`. Because every reader holds a read turn from before it opens the index until after it closes it, no file is ever removed while a pbrain process has it open. Ordinary use never removes a file: an index is updated in place and a superseded profile is deleted as rows inside it (10.4). pbrain never touches another checkout's files.
8. A restricted worker under a supervisor has no access to either root or to any ancestor it could replace; only the supervisor's broker has (spec 12.2).

**The lock mode** (#55). The pinned driver would choose between two Linux lock modes from an environment variable, `MODERNC_SQLITE_OFD_LOCK`. pbrain calls `sqlite.OFDLocking(false)` once, before its first open of an on-disk database. That call reports the previous mode, so pbrain checks its error and then asks `sqlite.OFDLockingEnabled()` and requires `false`; otherwise it fails with `io` before any database is touched. macOS has the conventional mode only. So both systems use conventional POSIX locks, whatever the environment says. Conventional was chosen because it is the one mode both have, and because the side file's coordination is conventional in either mode. @architect-2 checked the override on Linux with the variable unset, 0 and 1. The runtime audit of #55 is run again for every kind of database (memory, index, records, turn) and its exception is rewritten from "SQLite in memory only" to "SQLite on disk in pbrain's two roots, with this lock mode, `temp_store=MEMORY` and no other file".

**What remains.** A process of the same user that can replace these paths can redirect an open, side files included, between pbrain's checks and SQLite's calls. The checks catch what was planted beforehand and what belongs to another place; mode 0700 keeps other users out; the broker rule keeps restricted workers out. pbrain does not claim to confine a same-user process that is allowed to write there, and the spec says the same (12.6).

**Tests**, by name in the state component: two first creators of a records database and of a turn database at once, one killed before publishing, and one killed straight after, each ending with one file of one name that opens; a rebuild killed after the main file was removed and before its side files were, followed by an ordinary `sync` and a search, with no old side file opened beside the new database; side files with no main file where a memory, a records or a turn database should be, refused; a turn database found in WAL mode and a data database found in rollback mode, each refused; a damaged state records database with live memories and a recorded hooks install, refused and left as it is; an index of another place, sound, left alone; a link and a hard link at each name; wrong owner and mode; a real pbrain database of another project, of another checkout and of the personal section put in place, refused before any row is returned; an empty file at the name on a `store` and on a search, not adopted; the settings read back; the lock mode with the variable unset, 0 and 1 in fresh child processes; five writers and readers on Linux and macOS; a process killed before and after a commit, then `PRAGMA quick_check`; a read-only folder; a full disk.

### 6.6 Originals

`store.Tx.PutOriginal`, `RemoveOriginal` and the read of an original take the originals folder of 6.2 for the section's place, opened through the state root, where they took `<section>/.pbrain/originals`. Their checks are unchanged: a digest-named file, verified on write and before removal, and `delete`'s reference count over the items of this checkout's section, which is why originals are keyed by checkout (two worktrees can hold items for one digest on different branches, and neither can see the other's). An import stores the original before it publishes the item, and a delete removes the item before the original, so a crash leaves at worst an original nobody refers to. One correction to today's `delete` comes with this: if the removal of the item file could not be made durable (its folder's sync failed), the original is **not** removed.

### 6.7 Moving an existing `.pbrain/`

`store.Migrate(ctx, b, section)` is called by `store.Begin` and by `sync`, inside the section's turn (6.4), whenever the section's folder still has a `.pbrain` entry, with or without a receipt: without one it copies and cuts over, with one it only finishes the clean-up. A section that never had a `.pbrain` folder has no receipt and needs none.

The receipt is a row of `moved` in the state's records database, keyed like the originals folder it vouches for (project key, repository id and checkout id; or the personal id), listing the digests copied. It is committed in its own short transaction while the turn is held; the turn database itself stores nothing.

```
migrate(section):                                  # inside the section's turn
  if a moved row exists for this originals folder:  go to cleanup with the digests it lists
  old := open <section>/.pbrain through handles          # its old "lock" file is neither taken nor created
  for each file in old/originals named by a well-formed digest:
      copy to <state originals>/<digest> by temp + LinkNew (an identical file already there is kept)
      read the copy back; compare digest and length; Flush; sync the folder and each new parent
      any failure: return io                       # nothing old was removed; the command fails
  INSERT the moved row with the digests; COMMIT    # the cutover; synchronous=FULL makes it durable
      a failure here: return io; the next writer starts again at the top
cleanup:                                           # may run again after a crash
  remove exactly: old/originals/<digest> for each digest in the row; the files of old/index that
      carry the old index's names (manifest, lock, <16 hex>.db); old/lock
  remove old/originals, old/index and old only if each is then empty
  anything else found there is not pbrain's: leave it and its folder; the row stands
  after_write notice on a failure here; never copy again
```

Two brains in one checkout have two originals folders and two rows, and neither can skip its own copy on the strength of the other's. A personal section used from several checkouts has one row, under its personal id, and is moved inside its own turn by the first command allowed to write it.

Reads choose by one rule: an original is read from the old folder only while that folder exists and there is no row; in every other case, a fresh brain included, from the state folder. A read that finds no row and then fails to open the old file looks once more for the row and reads the state copy. Two first writers take turns; the second finds the row. An old pbrain, which knows neither the turn nor the places, must not write the same checkout or personal section at the same time.

## 7. Hosts: where the previous answer and its provenance come from

### 7.1 The seam

```go
package hosts

type Host interface {
    Name() string                                             // "claude"
    ParseEvent(event string, in []byte) (Event, error)        // the host's hook JSON
    Reply(event string, r Reply) ([]byte, error)              // in the form the host expects
    SessionRoot(home string) string                           // where this host keeps its session files
    Reader(rec SessionRecord) (SessionReader, error)          // ErrUnsupportedVersion when not on the tested list
    Settings() SettingsPlan                                   // what hooks install writes for this host
}

type Event struct {
    Kind        string // "session-start" | "pre-compact"
    SessionID   string
    Source      string // startup | resume | clear | compact | fork
    RecordPath  string // the host's own session file
    Dir         string
    Model       string // at this event; may be empty
}

type Answer struct {
    Text   []byte
    Turn   string    // the host's id of that message
    At     time.Time
    Model, Effort, Branch string // of that message; empty when the record does not say
}

type DialogAnswer struct {
    Turn      string
    At        time.Time
    Choice    string // "team" | "me"
    Selection string // the selection line the question carried
    Term      string // the term the question named
    Human     bool   // true only on a host version qualified to tell a person's answer from an automated one
}

type SessionReader interface {
    // PreviousAnswer returns the complete visible answer of the turn before the request, or ErrNoSource.
    PreviousAnswer(ctx context.Context) (Answer, error)
    // DialogAnswer returns the answer that was actually given to pbrain's question in the request's
    // own turn, whoever gave it; false when no such answer is in the record. Human says whether this
    // host version could tell that a person gave it.
    DialogAnswer(ctx context.Context) (DialogAnswer, bool, error)
}
```

`cmd/pbrain/main.go` builds the table of hosts, as it builds the model registry, so `hosts/claude` is imported in one place and tests can pass a fake host.

### 7.2 The session record

`pbrain hook session-start` writes one small file per host session, `<state>/.pbrain/sessions/<session-id>.json`. It is written at a start, never per turn.

```go
type SessionRecord struct {
    Harness, HostVersion string
    SessionID  string
    KeySHA     [32]byte  // SHA-256 of a random 128-bit key
    RecordPath string    // the host's session file, resolved, inside Host.SessionRoot
    Dir        string
    ModelAtStart string
    Context    int       // +1 on every clear, compact and fork: used to show the page once per context
    Task       string
    TaskSince  time.Time
    Started, Updated time.Time
}
```

Rules that keep it honest:

- **Only for a qualified launch.** The record is written only when the hook has established that the host launched it for this session and this brain (8.1). The following rules are checks on top of that, not a substitute for it.
- **The path is checked, not trusted.** `RecordPath` comes from the host's event. It is accepted only if, resolved, it is a regular file of this user inside `Host.SessionRoot`, its name is the session id's, **and the session it records ran in this project**: the folder the file's own entries name must lie in the work tree the event's folder opens the brain from. A session of another project, named in an event for this one, gets no record, no environment lines and an empty reply, and nothing of its own is changed.
- **Created once.** A record for a session id is created with `Dir.LinkNew`. A later start event of the same session (resume, clear, compact, fork) may replace it only if the process's environment carries that session's key. So a second caller cannot take over a live session's record by naming its id.
- **The key.** The hook appends two lines to the file the host names for the session's environment (for Claude Code, `CLAUDE_ENV_FILE`): `PBRAIN_SESSION` and `PBRAIN_SESSION_KEY`. A later `pbrain store` in that session finds the record by the id and accepts it only if the key hashes to `KeySHA`. A caller that sets `PBRAIN_SESSION` to another session's id has not that session's key and gets `no_source` without the other record being read.
- **What it is not.** The id and key reject a stale or wrong-context call. They do not authenticate a process of the same user, which could read another session's environment file; the spec claims no more (3.4). `Context` counts contexts for showing the page once; it is **not** eligibility for a checkpoint. A fork or a compaction starts a new `Context` and still carries what the old one had seen, so a supervisor's grant is bound to a worker it launched clean and is never renewed by a start event (the supervisor component, section 13).

Records older than 30 days are removed by the next `session-start` hook.

### 7.3 Reading Claude Code's session file

**What was seen.** One real session file of Claude Code 2.1.330 was inspected for its shape, keys only. It is JSON Lines. An assistant entry has `type`, `uuid`, `parentUuid`, `timestamp`, `sessionId`, `version`, `gitBranch`, `isSidechain`, an `effort` value on real model replies, and a `message` with `model`, `stop_reason` and `content` blocks of type `text`, `thinking` and `tool_use`. A user entry has `promptId` and, for a tool's result, `toolUseResult`. Host-made placeholder entries carry the model `<synthetic>`.

**What is not known.** None of this is a documented contract. The hooks reference documents the path of the file and warns that it may lack the final message at the moment a turn stops; it documents no schema.

**So the reader is narrow and gated.**

- It works only for host versions on a list compiled into `hosts/claude`. A version is added by a qualification run (section 11.4) whose evidence is committed. For any other version `Reader` returns `ErrUnsupportedVersion` and the command answers `no_source`.
- It reads at most the last 8 MiB of the file, as whole lines, and ignores every key it does not need. A line that should have a needed key and has not is a shape error: the whole read fails with `ErrNoSource`. It never guesses.
- **The request.** The newest user entry in the main conversation that is a prompt, not a tool result.
- **The previous answer, by ancestry and not by search.** Follow `parentUuid` from the request to the turn before it. That turn's last main-conversation assistant entry must be a real model's, must hold `text` blocks, and must have ended its turn (`stop_reason` `end_turn`). If the turn before the request was interrupted, failed, or ended in a tool call with no text, the answer is `ErrNoSource`: the reader never walks further back to find an older turn that happened to succeed, and never crosses to another branch of the conversation.
- The answer's text is its `text` blocks joined in order, byte for byte: no `thinking`, no `tool_use`, no tool results. `Turn` is its `uuid`, `At` its `timestamp`, `Model` its `message.model`, `Effort` and `Branch` its `effort` and `gitBranch` when present. `source_commit` is not in the file and stays absent.
- **Refusals**, each `no_source`: the entry's `sessionId` is not the record's; its `timestamp` is before `TaskSince`; the file's identity changed while it was read. Text over `TextMax` is `too_large`.

**The dialog.** Before it asks, the skill runs the store with `--dry-run` (a small addition to the spec's 5.1). The dry run checks a selection and admits nothing: it applies the limits and the secret scan to the text and resolves the message it comes from through the session record and the reader. It opens no section and no memory database, creates no state, checks no quota or duplicate, and spends no approval; the store that follows does all of that again. With no session record, or text that is not part of the request's prompt, it answers `no_source`; with `--fact` it is a usage error; it is not in the tool's schema. It prints a **selection line**: for "remember that X", the session, the id of the request's prompt, the byte range of X in that prompt, the length and the SHA-256 of X; for "your last response", the session, the id of the previous answer's turn, its length and SHA-256. The line holds no part of the text. A selection that fails a check, a planted secret included, is refused here, before any dialog and without being echoed.

The skill then asks its one question through the host's question tool with a fixed header, `pbrain: keep this for`, two fixed labels (team, me), a fixed phrase for the term (`long-term` or `short-term`), and the selection line. `DialogAnswer` accepts only a `tool_use` of that tool with that header that descends from the request's prompt in the same turn, together with the user entry that carries its result, answered no more than ten minutes ago.

**Two questions, kept apart.** *Was a matching answer actually given?* and *did a person give it?* are different. The record can show the first on any tested version. The second needs more: a program that drives the terminal can answer a dialog too, so a version is qualified for it only when something in the record, or in the host's documented behaviour, separates the two, with a negative case of the same shape answered by automation.

**What an answer covers, and how it is checked before anything is admitted.** In a session with a record, a store that names a person in `--asked-by` is a store at a request, and it is admitted only if `DialogAnswer` finds an answer and all of these hold:

- the selection line in the question names this session and this request, and the bytes it describes are the bytes being stored: for a span of the prompt, the reader takes that byte range from the prompt entry itself and requires it to equal the store's text, with the stated length and digest; for the previous answer, the turn and digest must be those `PreviousAnswer` returns. Any selection up to the store's own limit of 65,536 bytes works this way, with any characters and line endings;
- the term in the question is the store's term;
- the label that was chosen is the store's section: *me* with `--personal`, *team* without;
- the answer is fresh and unspent.

If there is no answer, or any check fails, the store is refused with `no_source` and writes nothing, in either allocation: an unanswered or cancelled dialog, an "answer" that is only text in a prompt, and a choice of *me* for a store that asks for the team all end here. When all hold, the store's transaction inserts the `approval` row (4.3) with the session, the dialog's id, the SHA-256 of the text, the term and the section: that row is the spending. It holds no text and outlives a `forget` of the memory for 14 days, so an unkeyed retry after a forget is refused as spent. A replay of the same keyed store is answered by its receipt before any of this and is not charged again.

**Who is recorded as having asked** (the owner's answer of 2026-11-26: an answered dialog is enough to store). When the checks pass and `Human` is true, `asked_by` is `observed` and the memory counts in the person's allocation. When they pass and `Human` is false, which is every version until one is qualified for it, the memory is stored all the same, with `asked_by` `claimed`, in the ordinary allocation. The owner's answer changes what the origin of a real answer is called; it does not remove the need for the answer or any of the checks above.

**Where there is no session record**, pbrain cannot see a dialog at all. The skill then treats the host as having no usable dialog: it stores nothing and gives the person the `store --stdin` command (spec 6.2). A person or a script that runs `pbrain store` at a shell is a deliberate, manual store and keeps the ordinary rule: `--asked-by` is recorded as `claimed` and counted in the ordinary allocation, because a person at a shell and an agent at a shell look the same to pbrain.

### 7.4 What a store looks like with a host

```
pbrain store --from-last-response
  flags parsed; --session and --capture refused with it
  record := sessions[PBRAIN_SESSION], checked with PBRAIN_SESSION_KEY        else no_source
  reader := host.Reader(record)                                              else no_source
  answer := reader.PreviousAnswer()                                          else no_source / too_large
  dialog := reader.DialogAnswer()                                            if --asked-by names a person: an answer must be found and
                                                                             match the selection, term and section, else no_source
  Row{Capture: verbatim, SaidBy: recorder, Turn, Observed: answer.At, Model, Effort, SourceBranch,
      Session, Harness, HostVersion; RequestTurn set from the dialog; AskedBy observed if dialog.Human, else claimed;
      each with its Basis}
  then the order of section 5.2 from step 4
```

A store with `--text` or `--stdin` in a session with a valid record gets `session`, `harness` and, when `--asked-by` is given, the dialog check. It gets no model or effort: those belong to a source turn, and only `--from-last-response` names one.

## 8. Hooks

### 8.1 `pbrain hook session-start`

Reads the event from standard input, bounded at 1 MiB. In order:

1. Parse; validate the event's session file path (7.2); write or replace the session record and the two environment lines.
2. **Decide whether this is the host calling, for this session and this brain.** An event on standard input proves none of that: anyone can run the command. Explicit recall and an unrequested page are different acts even when their bytes are the same, so the hook writes a session record, writes the environment lines and returns a page only on a host version **qualified** to establish all three of these positively:
   - that the command was launched by the host for the session the event names, and not merely handed an event;
   - that the environment file is the host's own for that session;
   - that the brain it opens is the one the hook was installed for (the `hook_install` row of 8.3 names the brain by its project key; with two brains in one work tree, an event that resolves to the other one is refused).

   How a version shows this is part of its qualification (11.4). Where it cannot, the hook returns an empty reply and writes nothing: no record, no environment lines, no page. Explicit `recall`, the paste path and a supervisor's own injection are unaffected; `--from-last-response` then answers `no_source`, since it needs the record. `PBRAIN_TASK` only selects which task's page; it grants nothing.
3. If so, memory is enabled and a task is set: run the page query (5.3) on the shared section and return it in the host's form (for Claude Code, `hookSpecificOutput.additionalContext`). The page holds shared memories only.

Any failure in any step produces an empty reply and exit 0; the session never waits on memory. The command gives itself 150 ms for the page, from process start, and returns an empty reply when that passes. The installed entry sets the host's own time limit to 5 seconds as a backstop.

Showing a page changes nothing else: a resume, a fork or a compaction does not extend any checkpoint eligibility and does not lift a pause.

### 8.2 `pbrain hook pre-compact`

Returns one fixed instruction on standard output, which Claude Code uses as instructions for its compaction summary: keep the current task token `<task>`, and note that the task's memory page will be shown again after compaction. No brain is opened and nothing is written.

### 8.3 `pbrain hooks install`, `uninstall`, `status`

`Host.Settings()` names the settings file (personal or shared) and the entries: for Claude Code two hook entries, `SessionStart` and `PreCompact`, each a command hook running `pbrain hook <event> --harness claude`.

What pbrain inserted into a settings file is a row of `hook_install` in the state's records database, keyed by the settings file itself (the identity of the folder that holds it, and its name): the path, the project key of the brain the hooks were installed for, the SHA-256 of the file before and of the planned result, the bytes of the file before (or "absent"), exactly the entries this operation inserts, the operation (install or uninstall) and whether it is `pending` or `done`. A personal and a shared settings file, and the files of two worktrees, each have their own row. The row holds a copy of a person's settings; it is in the records database below `.pbrain`, which is refused as an input (6.1).

No file edit can be undone by a database, so the order does the work, in two short transactions, the same for install and uninstall:

1. **Resolve what is pending first.** If the row is `pending`, finish that operation by step 3 before starting another on the file. An uninstall that arrives while an install is pending therefore first completes that install, or ends it as a conflict, and only then records its own intent.
2. **Transaction A: record the intent**, as `BEGIN IMMEDIATE` on the records database, so that the check for a pending operation and the new row are one step. Read the file (at most 1 MiB). Not valid JSON, or a duplicate key on the path to `hooks` or in the event arrays being changed: `invalid`, nothing written. Compute the result. An entry already present and identical is left alone and **is not recorded as pbrain's**; only entries this run inserts are. Without `--yes`: print the change and exit 1 with `confirm_required`, having written nothing. With it: write the row as `pending`, with a fresh operation id, and commit. Two cases write no row and change nothing: an install that finds every entry already present, which is a repeat and must leave the existing row, with its record of the original file, exactly as it is; and an install on a file whose row names another brain, which is a `conflict`. A run without `--yes` never completes an earlier pending operation either: it reports that one is pending.
3. **Transaction B: act on the intent**, as `BEGIN IMMEDIATE` on the records database, which keeps a second run on any settings file out for these few milliseconds. Read the row afresh, require it to carry the operation id this run recorded or is resuming, read the file afresh, and go by the file's hash:
   - it is the "before": write the planned file by temp, flush and replace, and sync its folder, before the row is marked done. The hash is read immediately before the replace; an editor that writes the file in the instant between is outside what pbrain can coordinate, as it takes no part in the transaction;
   - it is the "after": the file was already written by a run that died; nothing to write, but its folder is synced again before the row is marked done;
   - it is anything else: someone edited the file in between. Leave it, mark the row `conflict`, commit, and report `conflict`. The row is not marked done.

   In the first two cases, mark the row `done` and commit.

**Uninstall** plans the reverse in step 2. If the file's hash equals the recorded "after" of the install, the plan is the recorded bytes exactly, or the file's removal if it was absent. Otherwise it parses the file: for each recorded entry, an identical entry is removed; an entry with pbrain's command that differs is a `conflict` and nothing is changed. Entries pbrain did not insert are never removed, and every other key is kept.

The JSON is handled as an ordered tree of raw values (`encoding/json` with `json.RawMessage` and the key order), so untouched values are written back byte for byte. Only an uninstall of an untouched file preserves the file's formatting as a whole, which is what the spec promises. `status` reports the row, and says so when an operation is still pending.

## 9. Promotion

### 9.1 The parts

`memory.Root` keeps the preview, the token and the lock for a confirm; the existing item writer publishes; `cli` joins them.

- **For an item**, nothing new is needed in `store`: `Tx.Publish` already takes a reserved id and emitted bytes and writes exclusively. The checks that must run under the destination's lock (audience and route, links, duplicates) live in `cli.publishNew` today; that function is split so that its locked validation can run on a frozen draft without emitting again: `validateNew(ctx, wc, tx, draft) error`, then `tx.Publish(ctx, kind, id, bytes)`.
- **For a new document**: `store.CreateDoc(ctx, b, addr, data) (Commit, error)` opens the document folder through handles, refuses a path that is pruned, exists, or is not under an enrolled folder, writes by temp, flush and `LinkNew` (exclusive), and syncs the parent; a parent folder it had to create is synced in its own parent first.

### 9.2 Preview

1. Resolve every source id in the selected section. Missing, expired or out of section: `not_found`. Any report: `usage`. More than 16: `usage`.
2. Check the sources rule: each has a portable ref or is quoted. Portable is decided by form and one cheap check: a URL by its scheme; a path by `gitx.Tracked`; a commit by `git cat-file -e <id>^{commit}` (a new read-only query); an item id by a lookup in the scanned catalog of the same section.
3. Build the draft once. For an item: `cli.runAdd`'s validation with the body (the memory's text, or the author's text) plus the generated Sources part, then `item.Emit`, with the id reserved by `item.NewID` and checked free in the target checkout. For a new document: the header pbrain writes and the body. At most 1 MiB.
4. `Root.Preview`, in one write transaction: insert the `promo` row with the SHA-256 of a random 256-bit token, the sorted source ids, the target, the draft and its digest and the expiry; cancel an older unspent preview of the same source set; cancel the oldest unspent one beyond 32.
5. Print the token, target, length, digest and the first page. `--show` reads through `Root.Show`, which in one read transaction checks the section, that the token is unspent and inside its time, and that every source row is still there and visible, before it returns a byte of the draft.

A review patch (`--update`) stops after step 3 with the patch from `difflines`; it stores nothing.

### 9.3 Confirm

A database transaction cannot commit a Git file with it, so the order and the locks carry the guarantee. The order is fixed and used nowhere in reverse: the destination section's turn (6.4), then the memory database's write transaction. Two confirms of one token have the same destination, so they take turns there, and no separate lock is needed.

```
Root.Confirm(token, ids, publish):
  read p := promo[sha(token)]; ids, section, expiry            else not_found / conflict
  tx := store.Begin(p.target section)                          # the destination's turn, held to the end
  read p again                                                 # another confirm may have finished meanwhile
  if p.spent:
      return reconcile(p)                                      # never publishes, never reads a source
  validateNew(tx, draft): author, route, audience, links, duplicates, checkout identity, the id still free
      differs from the preview  ->  refuse; nothing spent, nothing published; a new preview is needed

  txn A (BEGIN IMMEDIATE):                                     # the spend
      p unspent and unexpired; every source present, visible, not expired, not a report
          else refuse; nothing spent
      p.spent = 1
  COMMIT                                                       # durable (synchronous=FULL): the barrier
      the commit failed or its durability is not known  ->  io "the token may have been consumed"; return

  txn B (BEGIN IMMEDIATE):                                     # no forget or eviction can commit while it is open
      stage the draft: temp file written and flushed            # the slow part, before the decision
      authorize(): read the clock now; every source present, visible and not expired at that instant
          else remove the temp; COMMIT with nothing published; return "a source was removed"
      link the staged file exclusively under its name          # the file's commit point, straight after authorize()
      only if the link succeeded: p.published = 1; p.draft = NULL
  COMMIT                                                       # a failure here, after the file exists, is after_write
  tx.Close()

reconcile(p):                                                  # against the frozen target only
  the target file exists and its SHA-256 is p.draft_sha  ->  acknowledge it
  else                                                   ->  conflict, outcome indeterminate; write nothing
```

**The publication is authorized at one instant:** `authorize()`, which runs after the bytes are staged and flushed and immediately before the exclusive link, with the write transaction and the destination's turn held. A `forget` or an eviction cannot commit while B is open, but time passes all the same, so the expiry of a source is judged by a fresh reading of the clock at that instant and not when B began. `Tx.Publish` and `store.CreateDoc` each take the authorization as a function they call between staging and linking: `PublishAuthorized(ctx, kind, id, data, authorize func() error)`. A `forget`, an eviction or an expiry that takes effect before that instant wins: nothing is published, and the spent token is not usable again. A `forget` that arrives while B is open waits on SQLite's write lock for the few milliseconds the publication takes, and then removes a memory whose item is already published, which the spec allows: the original and the item are independent from then on. Writers that wait longer than their limit get `memory_busy`.

This gives each row of the spec's table (10.1). Two confirms at once: the second waits for the destination's turn, reads the promo again, finds `spent`, and acknowledges the file. A crash after the spend and before the file: `spent` and no file, indeterminate, nothing written. A crash inside B after the file exists: `spent`, not marked published, and the file with the draft's digest: acknowledged. A lost reply after the file: the same, also after a `forget` of a source, because `reconcile` reads only the promo and the file. A commit of the spend that cannot be confirmed durable: nothing is published, and a retry finds `spent` and no file. A destination that changed while the confirm waited for the section lock is caught by `validateNew`, which runs under that lock and before the spend. `draft_sha` is kept until the token's expiry so the acknowledgement can compare.

### 9.4 The Sources part and patches

The Sources part is generated by `memory` from the rows, in the order the ids were given: a heading `## Sources`, then one entry per memory with who said it, who asked, the date, the capture mode and the ref; a quoted source is followed by its whole text as a block quote. It never prints a memory id, a session id or a recording path.

`difflines.Unified(a, b []byte, path string) []byte` writes one deterministic patch. It computes a line diff with a bounded effort (inputs over 20,000 lines, or a diff that exceeds its step limit, fall back to a single hunk that replaces the whole file, which is still a valid patch for review), and handles CRLF, a missing final newline and paths that need quoting as `git` does. Tests feed its output to `git apply --check`.
## 10. Documents and the indexes

### 10.1 The model

```go
package doc

type Address string // the path from the project root, forward slashes: "docs/plans/x.md"

type Document struct {
    Address   Address
    Title     string   // header title, else first heading, else file stem
    BodyStart int      // byte offset of the body in the file
    BodyLine  int      // file line on which the body starts
    FileLines, FileChars, BodyChars int
    Header    HeaderState // none | read | broken
}

// Parse splits a file into its header and body by spec 11.3 and never returns an error
// for a header: a broken header yields HeaderState broken and a body that is the whole file.
func Parse(addr Address, data []byte) Document
```

The header is read with the YAML library the project already has, for one key only, `title`, and only as a single-line string. A header is "well-formed" when the first line is `---`, a later line is `---`, and what lies between parses as a mapping; anything else is `broken`. The body starts after the closing line and one following empty line if there is one.

### 10.2 Configuration and the walk

`brain.Config` gains `Docs []string` and `Recall Limits`, parsed and kept as written. **`brain.Open` does not validate the folders**: an invalid `[docs]` table must not break `search`, `add` or any other item command. Validation happens in `DocFolders`, which only the `docs` commands call.

```go
package brain

type DocFolder struct {
    Path   string    // from the project root
    Pruned []string  // the managed area, an old .pbrain folder, and anything else cut from this folder's walk
}
// DocFolders resolves the project root with gitx.Top and applies spec 11.1.
// invalid_config for a bad table; git_required for a brain outside Git.
func (b *Brain) DocFolders(ctx context.Context) ([]DocFolder, error)

package store

type DocCatalog struct{ /* documents, skipped files with reasons, collisions */ }
func ScanDocs(ctx context.Context, b *brain.Brain, keep BodyPolicy) (*DocCatalog, error)
func InventoryDocs(ctx context.Context, b *brain.Brain, folders []string) (Inventory, error)
```

`ScanDocs` reuses the guarded walk of `store.Scan` (pbrain architecture 7.1): open the folder handle, list entries, skip dot folders and links, ask Git once for the ignored paths. It adds one rule first: a path that is, or lies under, an entry of `Pruned` is not opened, not listed and not counted. The managed area is always in `Pruned` when it lies inside the folder; the personal section lies inside it in the default layout and is cut with it. After pruning, any remaining path that the exposure guard knows as personal fails the whole scan with `personal_exposed`.

`InventoryDocs` is the same walk for folders that need not be configured. To report "not UTF-8" and "too large" it stats every eligible candidate and reads it, bounded at the 4 MiB limit plus one byte, as the scan would; it reads nothing under a pruned root and reports that root by its configured path only.

### 10.3 Search over both classes

The existing entry point is untouched: `search`, `list` and `get` call exactly what they call today.

```go
package search

type DocCorpus interface {
    Documents() []*doc.Document
    Body(d *doc.Document) ([]byte, error)
}

type MixedRequest struct {
    Request                 // today's request, unchanged
    Class   Class           // both | item | document
}

type MixedHit struct {
    Class Class
    Item  *Hit              // today's hit, when Class is item
    Doc   *DocHit           // path, title, snippet, location fields
}

// RunMixed takes what Run takes today (the corpus and the graph of the items) plus the documents.
func RunMixed(ctx context.Context, req MixedRequest, items Corpus, g *graph.Graph, docs DocCorpus, sem Semantic) (MixedResult, error)
```

- `Class item` calls today's `Run` and returns its result relabelled; the documents are not walked and no document index is asked for. `Class document` does not scan items.
- For both: `RunMixed` runs today's item search to its full ordered list, then matches the documents with the same matcher over title and body, and merges.
  1. Each hit has a match class, computed as today. A document has no tags, so its second class is "every clause in the title", the same as its first.
  2. Inside a class: items in today's order; then documents by BM25 over the document set (k1 1.2, b 0.75, the clause counted in title and body), then by address.
  3. The decision-in-force exception is applied by the item search before the merge and is not disturbed by it.
  4. An item-only filter (kind, tag, status, a time window) empties the document side and counts what it left out.

Because the item list is computed by the unchanged code and only then merged, `docs search --class item` returns today's list exactly, which is the first rule of the quality gate (spec 17.4). The graph, replacement and standing stay on the item side; a document never enters them.

Semantic results: `Semantic` is asked once per class that is searched and has an index; the two score lists are merged by score after the exact results, each hit marked with its class.

### 10.4 The indexes, on disk

**One engine, two kinds of entry.** Today's build, catch-up and source carry item-specific data (`SectionInput`, item paths, item hits). The passage and vector core is separated from that behind a neutral entry, and items and documents each get a thin adapter:

```go
package semantic

// Entry is what the engine indexes: one item or one document.
type Entry struct {
    Key     string   // an item's path in its section, or a document's address
    Title   string
    Body    func() ([]byte, error)
    Check   ContentCheck // crc and size of title and body, as today
}
type Corpus interface{ Entries(ctx context.Context) ([]Entry, error) }

type Kind uint8 // KindItems | KindDocs

func BuildKind(ctx context.Context, opt Options, kind Kind, at IndexDir, c Corpus, dl embed.Downloader, progress ProgressFunc) (BuildResult, error)
```

The item adapter wraps today's `SectionInput` and keeps item citations and the graph on its side; the document adapter wraps `store.DocCatalog`. `Passages(title, body)`, the fingerprints, the vector cache and the scoring statement are unchanged.

**The database.** One SQLite file per index folder, `index.db`, opened in place through `guarded.OpenDB` (6.5) in WAL mode with the settings of 4.4, and reused for the life of the checkout. There is no file per build and none per model. Its size limit is today's, per profile, not memory's. Today the limit is 2 GiB of the index file, so it is a limit on **pages**, and it stays one: a profile may occupy up to 2 GiB of database pages. Pages are not the bytes of the rows. @lead-dev measured, with these tables and indexes, 4,096-byte pages and 768-float vectors, 1.322 bytes of pages for each byte of row content, before item rows. So pbrain charges each profile in pages, by an estimate per row for the profile's dimension that the component document derives and a test proves to be at or above the real page count; a build that would pass 2 GiB of pages for its profile fails with `too_large` and leaves what was committed in use, as today. A staged profile must be able to finish beside a full active one, or the cutover that frees the old rows could never happen: so `max_page_count` is set for two full profiles and a margin, 4.5 GiB of pages, and because each profile is held to 2 GiB of pages that ceiling is never the first limit reached. The write-ahead log is outside `max_page_count`, and short transactions alone do not bound it: a reader that keeps an older snapshot open across many of a build's windows stops SQLite from starting the log again, and the log grows by every window until that reader ends. pbrain's own readers are single commands that end, so this needs a stuck or very slow one. The first release claims no limit on the log's size. The build asks for a checkpoint when it ends, and phase 4 measures the highest log size with a reader holding an old snapshot across many windows; whether a limit is needed, and which, is decided from that figure and brought to the owner with it. A disk that really is full fails the window in progress with `io`; that window is rolled back, everything committed before it stands, and the active profile goes on answering.

**Profiles inside it.** Today's `item` and `passage` tables assume one profile per file. They gain a profile:

```sql
CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL) WITHOUT ROWID;
-- identity: kind, project key, repository id, checkout id (or personal id), corpus, format = 4
-- active = the id of the profile that answers, or absent

CREATE TABLE profile (
  id INTEGER PRIMARY KEY,
  tuple TEXT NOT NULL UNIQUE,      -- the compatibility tuple, canonical: effective fingerprint, request marker, format
  fingerprint TEXT NOT NULL, model TEXT NOT NULL, dim INTEGER NOT NULL, request TEXT,
  state TEXT NOT NULL,             -- 'building' until a build finished with nothing missing, then 'complete'
  completed_at INTEGER,
  revision INTEGER NOT NULL        -- raised in every transaction that changes this profile's entries
);
-- item and passage as today, each with profile_id REFERENCES profile(id) ON DELETE CASCADE in its key:
--   item:    UNIQUE (profile_id, key)        (was: path)
--   passage: UNIQUE (profile_id, item_id, ord), and the vector's length checked against that profile's dim
```

The **compatibility tuple** is everything that today makes two manifests incompatible: the effective fingerprint, the request marker (such as `cuda_precision_request`) in a canonical form, and the format. The effective fingerprint alone would not do: a default fp32 build and an fp16 request that fell back to the CPU have the same effective fingerprint and different markers (`TestManifestProfileMismatch`). Lookups, the length check and the scoring statement are all scoped to one profile, so two profiles of different dimensions never mix. Vectors are still shared by the effective embedding profile alone, in the vector cache, as today.

At most two profiles exist: the active one and one being staged.

**Which profile a search and `sync --status` use**, with no device asked, as today. The `profile` rows are given to today's profile check (`manifestProfile`, which accepts the requested precision marker and the compatible CPU and Metal profiles), exactly as a manifest is today. If the active profile passes, it answers. If it does not and a staged profile passes, that one answers as far as it is built, with `semantic_partial`: this is the first build, which has no older profile to prefer. Otherwise there is "no index" for this configuration.

**A build**, replacing "Build" and "Publishing" of the semantic component:

1. Take the index's turn (6.4): an explicit `sync` waits for it after its progress notice, as today, and a search's catch-up makes one attempt and gives up. It keeps two builders from doing the same embedding twice.
2. Open the database, creating it if there is none. Find the profile row for the configured tuple, or insert one as `building`. If another staged profile exists that is neither this one nor active, delete it first: a second change of configuration abandons the first.
3. Read the entries and that profile's stored checks in one read transaction; compute what is missing, changed and gone, as today.
4. **Embed outside any write transaction.** For each window of passages: look up or compute the vectors, then one short `BEGIN IMMEDIATE` transaction that deletes the entry's old rows of this profile, inserts its new ones and raises the profile's `revision`, and commit. One entry's rows are never split across commits.
5. Gone entries are deleted in a final short transaction. If nothing is missing, **the cutover**, in one transaction: mark this profile `complete`; if it is not the active one, set `active` to it and delete the previously active profile, whose items and passages go with it. A `PRAGMA wal_checkpoint(PASSIVE)` ends the build.

**What readers see** is what spec 12.6 promises:

- Every commit of step 4 on the active profile is a usable checkpoint. A search takes the index's read turn, opens the database, takes one read transaction, and scores the entries of the chosen profile whose stored check equals the check of the file it just read; the rest are "not covered", with today's `semantic_partial` and counts. That is today's coverage rule, unchanged.
- An interrupted build left committed windows; the next build finds them in step 3 and embeds only the rest. Vectors it had fetched are in the vector cache as today.
- **Another model or profile is staged beside the active one, in the same database.** Until the cutover the active profile answers for the configuration it was built for, and nothing staged answers in its place. At the cutover the superseded rows are deleted in the same transaction: a search that began before it finishes on its own snapshot, one that begins after it uses the new profile. No file is left over, and the database may keep its size, since SQLite reuses the freed pages. If the configuration is switched back later, the old profile is built again as a staged one, taking every matching vector that is still in the cache and embedding only what is not.
- A build killed before the cutover leaves the active profile answering and the staged one resumable.
- The bounded catch-up of a search (semantic component, "Catch-up in a search") becomes the same step 4 with its existing bounds. The search remembers the profile it chose and that profile's `revision` from its read transaction. Catch-up makes one attempt at the index's turn; holding it, it reads the choice and the revision again, and if either differs a build has committed since the search looked, so it embeds and writes nothing, as today's catch-up does when the manifest has changed. Two different things can stop a search here. If it cannot take its **read** turn, because the state root cannot be read or no turn database exists, it cannot use the index safely and answers by words with `semantic_not_ready`. If it only fails to get the **build** turn for catch-up, because a `sync` holds it, it does not catch up and answers from what is committed, with today's `semantic_partial` where items are not covered: a running `sync` lets readers in, and that is the usual case. Catch-up never creates a database.
- A database that will not open, fails its identity check or returns an SQLite error is "unreadable" to a search, as today: the search falls back to lexical results with `semantic_not_ready`.

**Rebuilding an unreadable index needs no person**, as the pbrain spec promises today (9.4, scenario 35). `sync` removes and recreates it under the rule of 6.5 item 7, when no reader holds it, and refills it from the vector cache. If readers keep it busy, `sync` reports that and tries again next time.

**The personal index** is the same engine at `<cache>/.pbrain/index/v1/personal/<personal-id>/items/index.db`, with its vectors in `vectors-personal/`. A shared build never opens either; a personal build never reads the shared vector folder for a personal passage and never writes a personal vector into it. This keeps today's rule that a private note leaves nothing a shared checkout can reuse, in a new place.

**After the upgrade** the old `<section>/.pbrain/index` is not read. `search --semantic` reports `semantic_not_ready` until `sync` builds the new database, taking every shared vector it can read from the cache and embedding the rest. Old personal vectors lived in the personal `.pbrain` and are not carried over: the first personal `sync` embeds the personal passages again. The move of 6.7 removes the old index files.

**Why two databases for items and documents, and not one.** One would make `docs sync` and `sync` take the same turn and write the same file, and "each side reports its own readiness" would need bookkeeping inside one database. Two folders give that for nothing, and vectors are shared underneath by content.

**The warning when a build embeds on the processor** (spec 11.7). Four small changes, and none on the search path, which never reports a device, as today:

- `embed.Model` gains `CPURate float32`: passages a second, the lowest figure measured for that model on four cores, and zero where none was measured. This release: 1.11 for `embeddinggemma-2-q8-0`, 0.902 for `embeddinggemma-2`, and zero for the UD-Q6_K_XL row until it is measured.
- `semantic.Progress` gains `Pending int` and `Warn bool`. The builder reports `Warn` with `Run` and `Pending` at exactly two points, and a runtime report anywhere else (a writer that resolves its device before the plan exists, for one) never produces the processor warning:
  - **At the start:** immediately before the first text of this index is handed to the embedder (the first `flush` with a non-empty queue, after `openEmbedder`), if the runtime is the processor.
  - **After a fall from a GPU:** the Gemma runtime retries on the processor inside `Embed`, so the builder sees the changed `RunInfo` only when that batch returns. That boundary is the point: the whole batch counts as done.
- **What `Pending` is.** The distinct texts the builder plans to give the embedder from that point: what is still in the current queue after the batch just finished, plus the result of planning ahead over the entries not yet cut. Planning ahead is the builder's own step, not a second rule, and it is read-only: it works from the entries and bodies the builder already holds, reads no file of the tree again and writes nothing to the cache. `builder.cut` and `builder.reuse` run in a counting mode that keeps no text and no vector, so it applies the same `baseVectorByDigest` rule for entries being replaced, the same validation of a cached vector (length, checksum, unit length) through `Cache.readVector` for the profile's effective fingerprint, and the same aliasing of a digest that was already planned. It uses the same vector store as the build of that index, and no other: the shared vector cache for shared items and documents, and the selected personal section's own store (`vectors-personal/<personal-id>`) for personal items. A shared count never opens a personal store, and a personal count never the shared one. One difference from the build: `Cache.readVector` removes an entry it finds damaged, and planning must change nothing, so the cache gains a look-up that decodes and validates in the same way and leaves a damaged entry where it is; the build itself removes it when it gets there. It is done only when the runtime is the processor, so a GPU run pays nothing. When nothing is left, `Pending` is zero and the notice says so with no estimate.
- `cli.deviceNotice` takes the count, the rate and where the device choice came from (the flag or `PBRAIN_DEVICE`), and builds the one-line message of spec 11.7. The estimate is `ceil(pending / rate)` seconds, shown rounded up to whole minutes. The notice is shown once for each section and code, where today it is once for each code.
- **The new values travel the whole diagnostics path, typed.** `errs.Notice` and `cli.Diagnostic` (`catalog.go`) both gain five optional fields: `Section string`, `Reason string` (the `embed.Fallback` value as it is: `asked`, `unsupported`, `absent`, `failed`, `memory`), `Passages *int`, `PassagesPerSecond *float64` and `EstimatedSeconds *int64`, each left out of the JSON when unset. `emitNotice` copies them into the line it prints; `callDiagnostics` already decodes that line into `Diagnostic`, so `Result.Notices` and the live `onNotice` of `Call` carry them with no further change, which is what the consumers planned in #81 see. `diagnosticSchema` (`schema_output.go`), whose notice object is closed, lists the five as optional properties of a notice, and not of an error. Text output is unchanged in form: the code, a colon, the message. A notice that sets none of them prints exactly the bytes it prints today.

### 10.5 `docs check` and `check`

`check.Docs(cat *store.DocCatalog) []Finding` builds the findings of spec 11.5 from the catalog's skipped files, collisions, broken headers and a secret scan of each document. `check`'s existing function is not called with documents and its output is unchanged.

## 11. Errors, tests and measurements

### 11.1 Errors

New codes in `errs`: `memory_not_enabled`, `memory_busy`, `secret_refused`, `quota_full`, `no_source`; new notices `docs_not_configured` and `state_moved`. An SQLite "busy" past the limit is `memory_busy` for memory and "waiting" as today for an index; any other SQLite error is `io` with SQLite's code in the message and never a caller's text. Every new command is registered in the command table with its error and notice sets, so `pbrain schema` lists them and the output-contract tests of the first project cover them.

Two rules hold everywhere: a refusal before the commit point writes nothing and prints nothing on standard output; a failure after the commit point is an `after_write` notice with exit 0. A hook never exits non-zero.

### 11.2 Kinds of test

| Kind | What it covers | How |
| --- | --- | --- |
| Unit | `memory` (admission order, replay, caps, allocations, expiry, the word index against the full scan, labels and the keyword rule, the lists and their cursors, the dropped count, the page), the `browse` model, `render.Inert`, `doc.Parse`, `difflines`, the host parsers, `guarded.Roots` and `Bind` | Table tests with a fixed clock and a seeded random source, on real temporary databases |
| Fault | Every step of the move, of a confirm and of `hooks install` | The replaceable `sys.Files` of the first project, failing each call in turn; for SQLite, child processes killed at named points and a full-disk file system |
| Concurrency | Five writers and readers of one memory; two confirms; two first writers of a legacy brain; a reader during an index build; two checkouts writing one personal section, and two worktrees writing their own brains without waiting | Real processes on a real file system, Linux and macOS |
| Scenario | The 55 scenarios of the spec | The existing scenario harness in `cmd/pbrain`, one test per scenario, named by its number |
| Privacy | With and without planted personal data, shared output, files and messages are identical; a previous root's personal original is refused as input | The existing personal-overlay method, extended to both roots and the document walk |
| Contract | CLI and tool give the same result | The existing `TestCallMatchesCLI`, extended to the new tools; arrays in `Bind` |
| Audit | The runtime audit of #55, with the two databases | Its existing harness; the exception text of 6.5 |
| Gate | The document quality gate | The calibration tool, with the frozen fixture and its hash check first |

The cases the reviewers asked for by name, grouped. Each is a test in the component document that owns it.

- **Quota and replay.** The ordinary allocation full by count, and separately by bytes; the person's allocation full by count, and full by bytes before its 2,000th row (1,024 texts of 65,536 bytes are exactly 64 MiB, then one more byte is `quota_full` while an ordinary store still succeeds); the ordinary allocation and the short-term limits full together, with a store at a person's observed request still succeeding; a duplicate that is not charged twice, also against a full allocation; a retry from another worktree after a lost reply; a changed ref, end date, requester, tag or run under the same event (`conflict`); 32 full task buckets making 1,024, of which the 24 oldest go; each removal by a count limit adding one to `dropped` for its hour and reason, and a removal by age adding none.
- **Admission.** A secret planted in a branch name; a late write from a session whose record was replaced; a dry run that opens no memory in either section; a planted secret in a selection, refused by the dry run with neither the dialog nor the refusal echoing it.
- **Approval**, on one host version that is not qualified to tell a person from automation: a dialog really answered, stored as claimed and ordinary; a dialog unanswered or cancelled, and an "answer" that is only text in a prompt, each storing nothing; *me* chosen while the store asks for the team, refused; words from elsewhere in the prompt offered as the selection, refused; an answer used twice, with a changed text, and again after a `forget`, each with no new row; an approved selection of 1,001 characters, one of exactly 65,536 bytes with Unicode and CRLF, and one a byte over. On a qualified version, the same answered dialog is observed and counted in the person's allocation. With no session record, the skill stores nothing and a manual store is claimed.
- **Hosts.** A forged hook event naming a file outside the host's session root, and one naming a real session of another project; an event that the hook cannot tie to a host launch, and one for the other brain of a two-brain work tree: each with no record, no environment line and no page.
- **Identity.** A real pbrain database of another place, including the other brain of the same checkout; a folder whose inode and device recur with another birth time; two rebinds at once; `.git` replaced while the checkout and brain folders stay, after which the first `import` and `sync` work and inherit neither the old originals nor a finished move; an unrelated empty file where a database is expected, never adopted as a memory.
- **Enabling memory.** `memory init` in worktree A, a store in B and a recall in A; init killed after creating its file and after setting WAL, then run again; a second init on a memory in use, changing nothing; `memory init --personal` from two clones at once, killed in one before its identity rows and run again from the other, ending with one personal database and no shared database opened; init from clone A and later from clone B on the same personal folder, with the enrolment and the database unchanged and both clones reading the same memory; two clones with default personal sections staying apart.
- **Turns.** Two ids that differ only by capitals, and one id in two kind folders, added at once: one succeeds; two different successors of one decision; an import against a `delete` of the last reference to the same original; two `audience` edits, neither lost; two clones writing one personal section; a turn's holder killed before and after its file was published, and the next writer going ahead; a wait that ends at its limit with nothing changed; a memory store that is not delayed by a slow `sync` or a model download; two projects asking for the same model at once, with one download; two processes that resolve one state root, also through a link, taking turns; the state root unwritable, with `add` failing before any item is touched.
- **The move.** Two brains in one checkout with different originals, moved one after the other, with a foreign file in each old folder; a move whose receipt could not be committed; a kill after the receipt and during clean-up; a `delete` whose item removal could not be made durable, leaving its original; a `delete` killed after the item was removed and before the original was, leaving an original nobody refers to and never an item without its original.
- **Promotion.** A `forget` during a deliberately delayed confirm, and separately a fake clock moved past a source's expiry after transaction B began and before the exclusive link, each with no item; the rows of the spec's failure table (10.1).
- **Indexes.** With the profile limit and the real page limit both set low for the test, and vectors of each supported dimension: an active and a staged profile that each fit and together exceed one profile's limit, completing the cutover without reaching the page ceiling; a staged profile that is itself over the limit, failing with `too_large` and leaving the active one answering; the page estimate never below the pages SQLite reports; a full disk in the middle of a staged build, leaving the active profile answering; `sync --status` held open across an exclusive rebuild, which waits for it; the rebuild in two cases that are told apart: with no other reader, an ordinary `sync` that finds the damage while it holds its own read turn completes the rebuild, which proves it let go of its own connection and turns first; with one outside reader holding the read turn, the rebuild waits for that reader alone and completes when it closes; a reader that starts during the clean-up, kept out until the new index is published and never seeing a file half removed; a search during a `sync`, answering from the index without catching up. A first build, an incremental one, and a staged profile, each after its first commit, after a kill and after a resume, with a reader running; a search that began before a cutover finishing on its snapshot; one index file for a checkout and kind after repeated changes of profile, and another checkout's index byte for byte untouched; a base profile completed, an fp16-request fallback staged beside it, and the configuration switched back; a CPU fallback for an fp16 request and a precision change, chosen without a device; a build that commits between a search's read and its catch-up turn (`TestCatchUpLock`) and a catch-up that never creates (`TestCatchUpNeverCreates`); an index with an unreadable header, and one with damaged pages and sound identity rows, each removed and rebuilt by an ordinary `sync` once no reader holds it, and left alone while a reader does.
- **The processor warning** (spec scenario 50), with a test model that has a GPU path, a device list and a rate made for the test: each of the five reasons, with the flag and the environment variable named apart; the count equal to the number of texts the test embedder is then given, on a fixture with a damaged cached vector, a vector held only by another entry that this run replaces, and a repeated digest; a cache filled by another process after the warning, with fewer texts given than announced; the shared count made with the personal storage unreadable; a personal index with one vector reusable only from its own store and one real miss, with the shared vector store unreadable, announcing one and embedding one; a damaged entry still in place after planning alone; the estimate and its rounding at 59 seconds, 60 seconds, 59 minutes and above an hour; a rate of zero giving the count and no estimate; one warning for each of the shared, the personal and the document index; the fall from a GPU to the processor in a middle batch and in the last one, where the batch that fell counts as done and the last case reports zero with no estimate; a writer that resolves its device on an unchanged index, with no warning; a failure and a cancellation after the warning, each with its usual error and exit code; nothing printed by a run that embeds nothing or by any search; the JSON line valid under `DiagnosticSchema`, equal between the command line and `Call`, on the terminal result and on the live callback, with both estimate fields absent when the rate is zero; an existing notice byte for byte as before. And for the index's record (spec 12.6): another model and a rejected precision, each refused with no vector scored and no device asked; a matching and a refused index in one search, with `semantic_partial`; today's `TestIndexProfilesAcrossSyncs` case, a shared fp16 profile and a personal processor fallback under one request, still answering together.
- **Labels.** The keyword rule on a fixed text in a fixed store, on Linux and on macOS with the same answer; a text of 10,000 distinct words, with at most 64 counts made; a store into an empty section; a report with none; keywords unchanged on a replay after other rows arrived; a shared row's keywords equal with the personal store full of its words and with it unreadable; a tag refused by the secret scan and not echoed; labels gone with a forgotten, an expired and a dropped row; `recall` in the same order with and without labels on the rows.
- **The lists.** Three rows received in one second, paged one at a time with forgets between the pages, each remaining row returned once; between two pages, a row inserted in the same second with an id below the cursor's, a row inserted after the clock was set back an hour, and in highlights a blocker inserted after the learned rows were paged, none of them on a later page and all of them in a fresh list; `rowid` not reused after the newest row is forgotten and another stored; a cursor refused with another filter, another section and another kind of list; every filter alone and all together, each kind of bucket among them; highlights with a row that fits two groups listed in the first only, reports, checkpoints, handoffs and short-term outcomes left out, and `Counts` equal to a count by hand; sessions with a run of three sessions, a session with rows inside that run and outside it, a session named `unassigned`, and rows with neither, where every row is in one bucket and each bucket's count equals its list's; more buckets than a page with three sharing their last second, each reached once; a bucket whose latest row is forgotten between pages, shown again with its remaining rows and none missed; a long-term handoff left out of highlights and a long-term outcome in group 3; a span resolved once and carried by the cursor, with `--since` beside `--after` refused; a cursor that does not parse; `Dropped` at a boundary hour, for a filter that matches no live row, and with a row older than 14 days and no write since; a label on a read with an accented letter, and one of 32 characters and more than 32 bytes, through the command line, the catalog and the screen; no personal row, count, label or session in a shared list, and the reverse; each list within its budget with the section at its limits.
- **The screen** (spec scenario 54), on the model without a terminal and once through a pseudo-terminal: paging, expanding, the exact view of a text with an escape, an OSC sequence and other control bytes, drawn as visible escapes that read back to the stored bytes, each filter, the three views, a bucket opened from Sessions with its period; forget and promote only after their confirmations, a decision with its decider, a long draft read page by page through `Show`, the result of promote equal to the command's, and a preview that is cancelled leaving no item and a token that lapses; a row that went meanwhile shown as gone; a text with control sequences, a terminal link and an image reference, drawn with nothing but colour and style sequences reaching the output; no connection open between two keys, shown by a second process storing during an idle minute; a run that only looks changing no memory, item, cache entry or record and keeping no history, with SQLite's own side files allowed to come and go (6.5), tested apart from the run that confirms changes; a forced failure with a marker in a memory and the libraries' debugging variables set, with the terminal restored, no marker printed and no file created; no terminal, which is `usage`; absent from the catalog.
- **Hooks install.** Install and uninstall in the personal and the shared mode and in two linked worktrees; install, install again, then uninstall, restoring the original file byte for byte; an uninstall arriving while an install is pending; a run without `--yes` while one is pending, completing nothing; a second brain installing into a file another brain owns, refused; a kill after the intent and after the file was written, and a failed sync of the folder straight after the rename, each resolved by the next run; an edit by someone else between the two steps, reported as a conflict and left; a settings file holding a made-up private value in ordinary words, after which a shared `import` and a shared `--body-file` aimed at the records database, directly and through a link, answer as for a missing file and create no item.
- **Placement.** No setting at all, on Linux and on macOS; the cache and the state root equal, nested, or the same through a link, refused with `invalid_config` before anything is created.

### 11.3 Measured in phase 1, in pbrain

- Store and recall at the limits of 4.5 (1,000 short-term rows with 8 MiB of text, 10,000 long-term rows with nearly 320 MiB, their tags and keywords, 50,000 receipts), one writer and five, as whole commands in fresh processes: 50th, 95th and 99th percentiles and the worst case, on Linux and on macOS. Nothing in 4.6 was measured at these limits: the earlier runs had 48 MiB of text.
- **The page-limit gate**, which decides the file ceiling and with it whether the reserve is a guarantee. With the real `max_page_count`, and texts of the densest shape for the word index (64 KiB of distinct three-letter words): fill the ordinary allocation and the short-term limits first, to both their rows and their bytes; then fill the person's allocation to its rows, and separately to its bytes. Every store inside its allocation must succeed. The ceiling is then set from the file that results, with a stated margin; until that run it is provisional.
- At the same limits with ordinary prose: the size of the database file and the highest size of its write-ahead log; a page of `memory log`, of `memory highlights` and of `memory sessions`; the keyword step of a store of 64 KiB of distinct words, for size, and of a text whose 64 candidates are each in nearly every row, for the dense counts, the second with five writers at once; and the ordinary allocation and the short-term limits full together with a store at a person's observed request still succeeding.
- Recall for a rare word, a miss, a word in every row, a query of several common words, and a phrase that only a late candidate holds; and stores of 64 KiB texts made of distinct words, for the size of the word index.
- The 100 ms recall gate of the spec (17.5) is judged on these. The scratch figures of 4.6 say the word index has room; they are not the gate.
- The session-start hook against its 100 ms budget at the caps.

### 11.4 Qualifying a host version

A host version is added to the tested list only by a recorded run on that version, on a real session, that shows: the previous answer is returned byte for byte for a plain answer, an answer after tool calls, and an answer with Markdown and trailing spaces; a side conversation and a tool result are never returned; an interrupted, failed or tool-only previous turn gives `no_source` and not an older answer; a changed task, another session and a forged key each give `no_source`; the model and effort are those of the answer's turn across a model switch; the task page arrives after a start, a resume, a fork and a compaction; and the dialog case of section 7.3 with its automated negative. The evidence, the host version and the commands are committed with the change that adds the version.

## 12. Risks, and one question for the owner

| Risk | What limits it |
| --- | --- |
| A same-user process redirects a database path | Stated in 6.5; pre-open checks, identity before use, the broker rule; not claimed to be eliminated |
| SQLite behaves differently on macOS (locks, side files, sync) | The same tests run there in CI before each phase is merged; no Mac measurement exists yet |
| A commit or a checkpoint has a long tail (one 415 ms store was seen) | No per-call latency is promised; the gates are percentiles; `busy_timeout` turns a long wait into `memory_busy` |
| Claude Code's session file changes shape | The version list; shape errors fail closed; the paste path always works |
| No Claude Code version can show that the hook was launched by the host for the session it names | Then the automatic task page and `--from-last-response` are not in the first release for a person's own sessions; explicit `recall`, paste and a supervisor's injection are. The owner is told with the qualification result |
| Every item write now needs the state folder, to take its turn | Stated in the spec (12.8) and the README; a command that cannot reach it fails before touching an item; the release note tells people to allow the folder in their sandboxes |
| Two processes are given different state folders for one checkout | They do not see each other's turns. Stated as unsupported in the spec (12.5) and the README; a supervisor resolves the folder once for all its workers |
| A reader that does not end lets an index's write-ahead log grow during a build, and the first release claims no limit on it | pbrain's readers are single commands; a full disk rolls back the window in progress and leaves the active profile answering (10.4). Phase 4 measures the highest log size with a snapshot held across many windows, and a limit is decided from that |
| The estimate in the processor warning uses a rate measured on another machine, and the UD-Q6_K_XL model has none | The text says "about" and shows the rate; a model with no figure gets the count and no estimate. Measuring that model on four cores is @lead-dev's, in phase 4. The extra pass that counts pending passages is timed there too |
| The limits allow seven times the text that was measured (328 MiB against 48), and the densest legal texts could need a file of more than 2 GiB for the word index alone | Nothing is promised from the old figures. The file ceiling is provisional at 4 GiB. Phase 1 runs the page-limit gate with the densest shape and every other gate at the new limits (11.3); the ceiling, or else the byte limits or the index, is corrected from that and brought to the owner |
| The newest 1,000 short-term memories can be fewer than a week's, or a night's | Stated in the spec (7.1). Each removal by a count limit is counted, and the lists say how many were dropped in the period shown; the pilot sets the numbers |
| A keyword is a guess from word counts, and the first memories of a store get plain ones | Shown apart from tags, never used for rank, term or quota; the rule is versioned, so a better one can follow for new rows |
| The four Charm libraries bring about 10 MiB, 28 modules, start-up environment reads and two ways to write a file at a variable's bidding | The audit's rule is kept. How the screen is shipped so that it holds is an open decision for the owner between two options (5.5), and phase 6 does not start before it |
| A memory's text reaches a terminal | `render.Inert` on every value from a memory, and a second filter on the Markdown renderer's output that passes colour and style only (5.5) |
| The cost of a turn on every item write is not measured | One open and one `BEGIN IMMEDIATE` on a tiny database; the memory store was about 5 ms with a full write. Measured in phase 4 with the rest |
| Moving originals loses one | Copy, verify, sync, receipt, and only then remove; faults at every step are tests |
| A wide document folder makes every `docs search` slow | The scan is parallel as today; the gate reports the time; enrolment is explicit |
| The factory side of checkpoints does not exist | pbrain's side is a contract (component 6); the page and explicit capture do not depend on it |

**What the owner decided on the open points** (2026-11-26 and 27): an answered dialog is enough to store a typed "remember", recorded as claimed and counted as ordinary until a host version is qualified (7.3); a deleted worktree's data is left in place and documented, with a command to list and prune it later; pbrain keeps no lock files and takes turns through SQLite (6.4); and a checkout has one index that is reused, with a superseded profile deleted by pbrain itself (10.4).

**Known only after qualification**, and to be reported to the owner then: whether a Claude Code version can show that the session-start hook was launched by the host for the session it names. If none can, the automatic task page and "remember your last response" are not in the first release for a person's own sessions; a paste and explicit `recall` are.

Not settled here, and owned: the factory's restricted worker, labelled channel and grant, to be designed in the factory repository; the per-host readers for Codex, Antigravity CLI, pi, omp and OpenCode, each when someone qualifies that host; and a command to list and remove the data of worktrees that no longer exist.

## 13. Component documents and build order

This project is large enough that the architecture is in two parts. Seven component documents follow, each with its full interfaces, algorithms and named tests:

| Component | Covers | Phase |
| --- | --- | --- |
| `components/state.md` | `guarded`: the two roots and `Roots`, the records database, places and `Bind`, `OpenDB` and its tests, the lock mode and the #55 exception, turns; originals in the state folder; the move of an old `.pbrain/` | 1 (roots, records, `OpenDB`, turns) and 4 |
| `components/memory.md` | `memory`: schema, admission, retention, replay, allocations, the word index, labels and the keyword rule with its stop list, recall, the lists and their cursors, the dropped count, the page; `store`, `recall`, `forget`, `memory init`, `memory log`, `show`, `sessions` and `highlights` in `cli` | 1 |
| `components/browse.md` | `browse`: the model, the views and keys, `Source` and `Actions`, the output filter, the dependency pins, their licences, the audit, and whichever way of shipping the owner chooses | 6 |
| `components/hosts.md` | `hosts` and `hosts/claude`: events, the session record, the reader, the dialog, `hook` and `hooks` in `cli`, qualification | 1 and 3 |
| `components/promotion.md` | Preview, token, confirm, the Sources part, document targets, `difflines` | 2 and 5 |
| `components/documents.md` | `doc`, the configuration and walk, `RunMixed`, the entry adapter and the on-disk indexes, the `docs` commands, `docs check`, the gate runner | 4 (indexes) and 5 |
| `components/supervisor.md` | The contract a supervisor must meet for checkpoints, the pause and the page: what it calls, what it must guarantee (a clean worker, a grant bound to it, no access to the two roots), what pbrain refuses regardless | 3 |

Build order inside the phases of the spec (17.1):

1. `guarded` with `Roots`, the records database, `Bind`, `OpenDB` and `Turn`, and `semantic.OpenCache` on it. Then `memory` with `memory init`, `store` (text and standard input, with tags, keywords and the run), `recall`, `forget`; then `render.Inert` and the lists, `memory log`, `show`, `sessions` and `highlights`; the catalog entries; the skill text; the measurements of 11.3. Then `hosts`, `hosts/claude`, `hook session-start` in its record-only form, `hooks install`, `--from-last-response`, and the qualification of one Claude Code version.
2. The split of `publishNew`, preview, confirm, the Sources part; then grouped promotion.
3. The page in the session-start hook, `pre-compact`; the supervisor contract; the hook-cost measurement.
4. The section's turn in `store.Begin`; originals in the state folder; the move; the entry adapter and the on-disk item index with profiles; the index's and the model's turns; the audit of #55.
5. `doc`, `ScanDocs`, `RunMixed`, the `docs` commands; the document index; `docs check`; the gate; document targets for promotion.
6. After the owner's decision on how the screen is shipped (5.5): `browse` and `memory browse`, with the pinned libraries, their licence texts and the audit run on every release build. It needs phases 1 and 2: the lists and `render.Inert` from the first, grouped promotion from the second.

Before two developers build in parallel, one change adds the exported types and signatures of sections 4 to 10 with empty bodies and proves they compile together, as the first project did.
