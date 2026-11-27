---
status: draft
---

# Functional Spec: pbrain-2

## 1. What pbrain-2 is

pbrain-2 makes pbrain the tool a software factory's agents learn with. It adds two things to the pbrain that exists today, and keeps everything that exists:

- **Memory.** A person or an agent can have something remembered, and the next agent can recall it. What proves useful is banked in Git, where the whole team finds it.
- **Documents.** Any Markdown file under a folder the project names is found by search, as it is, without pbrain front matter.

This document says what is built. It is a delta: the first iteration's functional spec (`specs/projects/pbrain/functional_spec.md`, called "the pbrain spec" below) stays in force, and its sections are cited by number. Where the two differ, this document says so. The owner's decisions D1 to D20 in `proposal.md` and the owner's answers of 2026-11-26 are its basis (section 19).

### 1.1 Three classes of record

| | Memory | Document | Managed item |
| --- | --- | --- | --- |
| What it is | Something a person asked to remember, something an agent learned, or an agent's handoff, outcome or blocker | Any Markdown file under a document folder | A decision, learning, observation, note or source (pbrain spec 2.5) |
| Written by | `pbrain store` | Anyone, with any editor | `pbrain add` and `import`, or by hand |
| Kept in | Local state on one machine, outside Git | Git, where its author put it | Git, in the managed area (the brain folder) |
| Identified by | A random memory id, `mem_` and 32 hex digits | Its path from the project root; a move changes it | Its file name, which pbrain never changes |
| Word for it | **said** | none | **banked**, with its status *provisional* or *verified* |
| Lifetime | Short-term: days. Long-term: until forgotten or its chosen deadline | Until someone deletes the file | Until replaced; history kept |

**Said, banked, verified.** A memory is *said*: by someone, at some time, at someone's request. A managed item is *banked*: someone wrote it up and committed it, which is all the word claims. *Verified* stays the status an item has when someone confirmed its content (pbrain spec 3), and is the only word pbrain uses for "checked". pbrain never labels a row "established": it cannot observe a review. A document gets none of these words; it is reference material, shown by its path. A typed report (section 3.3) is shown as "Unverified: SOURCE reports …".

**The loop.** `store` → `recall` → `promote` → Git review → found by every clone. Promotion is the only bridge pbrain offers from a memory to something durable and shared across machines.

### 1.2 Terms

- **Section.** *Shared* or *personal*, as in pbrain spec 5. Shared memory reaches the agents of one operating-system user on one machine that work on the same enrolled brain; nothing more.
- **Project root.** The top of the Git work tree that holds the brain folder.
- **Managed area.** The brain folder: where managed items live. Its rules are unchanged.
- **Task.** A short token naming a piece of work, such as `83` or `pbrain-2`: 1 to 64 characters of `a-z`, `0-9`, `.`, `_` and `-`, starting with a letter or digit.
- **Actor.** `human:<name>` or `agent:<name>`, as in pbrain spec 3. An actor is a claim, not proof of identity.
- **Host.** The agent harness a session runs in (Claude Code, Codex and so on). **Adapter:** the small piece pbrain supplies so a host can call it on its events (section 9). **Supervisor:** a program that launches agents and delivers their work, such as the factory; it is not part of pbrain (section 8.4).

## 2. What does not change

- Every command of the pbrain spec keeps its name, flags, ids, JSON, error codes, exit codes and **result set**: the same invocation on the same brain returns what it returns today, whether or not memory is enabled or a document folder is enrolled. Plain `search`, `get`, `list`, `prime`, `check` and `sync` (with `sync --status`) stay on managed items.
- One notice says more than it did: `cpu_fallback`, which `sync` prints when it embeds on the processor, becomes a warning with a count and an estimate (section 11.7). Its code and its reasons are the same. In JSON it gains optional fields, so the description of a notice in the tool catalog grows by those fields; every other notice is unchanged, byte for byte.
- The tool names and meanings in the tool catalog (`pbrain schema`) do not change. New commands are new tools. JSON only grows (pbrain spec 6.2).
- A brain with no memory enabled and no `[docs]` key answers every command exactly as today, with one planned exception: pbrain no longer keeps any file of its own in the working tree, so its local data moves once (section 12.4), and the semantic index needs one `pbrain sync` afterwards.
- The output budget, rendering rules, streams and exit codes of pbrain spec 6 apply to every new command unless this document gives another number. Messages never echo a suspected secret or anything outside the command's section.
- pbrain still never runs a Git command that changes a repository, and never writes into a file it did not create, with the exceptions of `verify` (pbrain spec 8.3) and `pbrain hooks install` (section 9.3).

## 3. The memory record

### 3.1 Fields

| Field (JSON name) | Meaning | Who supplies it |
| --- | --- | --- |
| `id` | `mem_` and 32 random hex digits | pbrain |
| `section` | `shared` or `personal` | the caller, per call |
| `kind` | `handoff`, `outcome`, `blocker` or `statement` (3.2) | the caller |
| `intent` | `learned`, when the text is something learned; absent otherwise (3.2) | the caller |
| `retention` | `short-term` or `long-term` (section 7) | the caller, or the kind's default |
| `expires` | When it ends; absent for long-term with no chosen date | pbrain |
| `text` | The memory, stored byte for byte | the caller |
| `capture` | `authored` (the recorder wrote it), `verbatim` (an exact copy of a source message), `checkpoint` (written at a supervisor's checkpoint prompt) or `report` (3.3) | the caller |
| `task` | The task it belongs to, if any | the caller |
| `ref` | What the next reader can check: a commit, a path, a URL, a CI run. One line, at most 2,000 characters | the caller |
| `said_by` | Who said the text | the caller; default the recorder |
| `asked_by` | Who asked for it to be kept; absent when the recorder chose by itself | the caller |
| `recorded_by` | Who ran the store, resolved as `add` resolves an author (pbrain spec 8.1) | pbrain |
| `observed`, `requested`, `received` | When the text was said or seen; when someone asked for it to be kept, if that was later; when pbrain stored it. UTC | the caller; the adapter; pbrain |
| `source` | The producer, a token such as `manual` or `factory` (`^[a-z][a-z0-9_-]{0,31}$`); default `manual` | the caller |
| `session`, `turn`, `request_turn` | The host session; the message in it that the text came from; and the message that asked for it to be kept | the adapter, or the caller for `session` |
| `harness`, `model`, `effort`, `source_commit` | The host; the model and thinking effort of the turn that said the text; and the commit checked out at that turn. No command-line flag sets any of them | the adapter only |
| `recorded_branch`, `recorded_commit`, `recorded_dirty` | The checkout the store ran in, at store time: branch name, commit id, and whether it had uncommitted changes. This is where it was recorded, which may not be where it was said | pbrain |
| `basis` | For each provenance field present: `observed` or `claimed` (3.4) | pbrain |

A field with nothing to record is absent; pbrain never guesses a model, an effort or a person's name. Nothing from a transcript, a prompt, an environment or a tool's output is kept as metadata.

### 3.2 Kinds

| Kind | What it is | Default term |
| --- | --- | --- |
| `handoff` | What the next agent on the task should do | short-term |
| `outcome` | What happened or was found. The default kind | short-term |
| `blocker` | What stops the task | short-term |
| `statement` | Something someone asked to have remembered | long-term |

**Learned.** Something learned, by a person or an agent, is a `statement` with `intent: learned`, set with `--learned`. What was learned is always long-term: `--learned --retention short-term` is a usage error, and so is `--learned` with any kind but `statement`. The mark says what the text is about, not how it was captured: a learned statement can be authored or verbatim. It is shown as said, like every memory, and the mark claims no check. It is a mark and not a kind of its own on the advice of both reviewing designers, so that nothing looks like the item kind `learning`; the owner's question on this is in section 20.

### 3.3 Typed reports

A typed report is a memory with fixed wording and no free text: `pbrain store --fact check-passed|check-failed|handoff-delivered`. It requires `--task`, `--source`, `--session`, `--event` and `--observed-at`. Every report, whoever stored it and whatever `--source` it claims, is unverified: results carry `"verified": false` and its source, and text shows `Unverified: factory reports check-passed for task 83.` A row whose label cannot fit the budget is left out, never shown without it. A report is short-term, ranks after every authored memory, cannot be promoted, and satisfies no release, promotion or review gate. There is no input that makes a report verified.

### 3.4 Observed, claimed, unavailable

Every provenance field is marked by where its value came from:

- **observed**: pbrain saw it (the store time, the checkout's branch and commit), or the installed adapter or a supervisor reported it from the host's own events (session, turn, model, effort, harness, a supervisor's member name).
- **claimed**: the caller stated it on the command line.
- **unavailable**: nobody could know it. The field is absent.

"Observed" is not authentication. On one user's machine pbrain cannot prove that a value came from the adapter and not from a caller imitating it, exactly as it cannot for a typed report. The word tells a reader how the value was obtained, and no field is ever an authority token.

Rules that follow:

- **Model, effort and harness** come from the host through the adapter, never from an agent's account of itself and never from a flag. The adapter's session-start hook notes the session's id and the model at that start (section 9.1). That establishes the model at the start only: for a later source turn pbrain records that turn's model if the adapter can read it, or the model from an observed history of switches, and otherwise leaves it absent. Effort and the source commit are recorded only when the adapter could read them for the turn that said the text. Without an adapter all of these are absent. The values are those of the source turn: a later model switch or checkout change is never attributed to an earlier answer, which is why the recording checkout has field names of its own.
- **A person as `asked_by`** is `observed` only when the adapter saw a positively identified interactive action by that person, such as an answer in the host's own question dialog. A prompt's role, a hook's text, a pane's name and a second unlabelled "yes" are none of them that. Otherwise it is `claimed`.
- **A byte-for-byte copy** is recorded as `capture: verbatim` with the source `session` and `turn`. It is never called verified.

### 3.5 Limits and refusals

- Text is at most 65,536 bytes of UTF-8 for `authored` and `verbatim`, and 2,048 bytes for `checkpoint`. Longer text is refused with `too_large`. Text is never cut, redacted, normalized or summarized: it is stored exactly or not at all.
- Empty text, text of control characters only, and text that is not valid UTF-8 are refused with `invalid`.
- The secret scan of pbrain spec 8.1 runs on the text and on every metadata value before anything is written, logged or printed, in either section. A match is refused with `secret_refused`, naming the field and never the value. The person can pick a smaller, clean excerpt. The scan is a second line of defence, not the privacy boundary: it cannot tell who may read a piece of prose.
- A success receipt never prints the text.

## 4. Where memory lives

### 4.1 The state folder

Memories are originals: nothing can rebuild them. So they are kept in the per-user state folder and not in the cache (section 12.2 has both).

- **Shared memory** lives in the state folder under the project's key: one SQLite database per brain, shared by every worktree of the repository.
- **Personal memory** lives in the state folder too, under the key of the personal section, in a folder of its own. A shared call never opens, counts, hashes or locks it, and nothing about it can change a shared result or error.
- **The project key** joins all linked worktrees of one repository for one brain: it is derived from the repository's Git common directory and the brain's path inside the work tree. A second brain in the same repository, a second clone of the same repository on the same machine, another user and another machine each have their own memory. Branch names and remote URLs play no part. A brain outside Git is keyed by its own resolved path.
- Clearing the cache loses no memory. Deleting the state folder loses all of it, and pbrain says so in `memory init` and in the README.
- pbrain writes nothing into Git's own metadata, and nothing of its own into the working tree.

### 4.2 Enabling memory

`pbrain memory init [--personal]` enables memory for the brain on this machine. It is a command for a person, not a tool: it is not in the tool catalog. It creates the folders, checks their owner and permissions, prints the state path, and prints the same project identity when repeated. Until it has run:

- `store`, `forget` and `promote` fail with `memory_not_enabled` (exit 1);
- `recall` succeeds with an empty page and a `memory_not_enabled` notice, so that a hook or a skill step that recalls never fails a session.

If the repository at that path has been replaced by another, memory must be enabled again and starts empty; the old memories are never shown to the new project.

### 4.3 Concurrency

Any number of agents in any worktrees may store and recall at once. A reader always sees a complete state, the one before or the one after a write. A writer that cannot get its turn within two seconds fails with `memory_busy`, which is retryable. A caller that may retry passes `--event` (a random UUID) and `--observed-at`: the same event with the same content returns the original id with `"duplicate": true`; the same event with different content fails with `conflict`; and a replay of a memory that was forgotten or has expired is answered "no longer available" and writes nothing. The retry guarantee has a horizon, as in the reviewed #83 plan: an `--observed-at` more than five minutes ahead or more than 14 days old is refused before anything else, and pbrain keeps a content-free receipt of every keyed store until 14 days after its observed time, also after a forget. So a late replay can never bring back a forgotten memory, a long-term one included: inside the horizon the receipt answers it, and past the horizon the store is refused for its age. A long-term memory can outlive the horizon; its retry guarantee does not.

## 5. Memory commands

All four are tools in the catalog: `pbrain_store`, `pbrain_recall`, `pbrain_forget`, `pbrain_promote`. `--personal` is given per call and never persists.

### 5.1 `pbrain store`

```
pbrain store (--text TEXT | --stdin | --from-last-response | --fact FACT)
             [--kind KIND] [--learned] [--retention short-term|long-term] [--until WHEN]
             [--task TASK] [--ref REF]
             [--said-by ACTOR] [--asked-by ACTOR] [--author ACTOR]
             [--capture authored|verbatim|checkpoint] [--source TOKEN]
             [--session ID] [--event UUID --observed-at TIME] [--personal] [--dry-run]
```

- **The text** comes from exactly one place. `--text` takes it as an argument. `--stdin` reads standard input to its end and keeps every byte, which also keeps private text out of the process list. `--from-last-response` asks the host's adapter for the previous complete, visible answer of the session (section 6.3). `--fact` makes a typed report (3.3).
- **Which entry point takes what.** The command line takes all four. A tool call takes `text` or `fact` only: a tool has no standard input and no host session of its own, so `--stdin` and `--from-last-response` are not in the tool's schema. An adapter or a supervisor uses the command line.
- **Combinations that are usage errors.** More than one text source, or none. `--fact` with `--kind`, `--learned`, `--retention`, `--until`, `--capture`, `--said-by`, `--asked-by` or `--ref`: a report has fixed wording, is short-term and is its own capture. `--until` with short-term. `--learned` with short-term or with a kind other than `statement`. `--from-last-response` with `--capture` or with `--session`: it is always verbatim, and its session is the adapter's, never the caller's. `--capture checkpoint` with a kind other than `handoff`, `outcome` or `blocker`, or with `--learned`, `--retention long-term` or `--until`: a checkpoint is a short-term note. These are refused before standard input or any source text is read.
- **A checkpoint** is stored by a supervisor with `--capture checkpoint`, which requires `--task`, `--source`, `--session`, `--event` and `--observed-at` and applies the 2,048-byte limit. Any caller may pass it; it only lowers the limit and labels the row, and grants nothing.
- **Kind and term.** `--kind` defaults to `outcome`, and to `statement` with `--from-last-response` or `--learned`. `--retention` defaults to the kind's term (3.2). `--until` gives a long-term memory a chosen end, in the forms of pbrain spec 7.1 for a date or an instant; it is a usage error with short-term.
- **Who.** `--said-by` is who said the text, default the recorder. `--asked-by` is who asked for it to be kept; leave it out when the recorder decided by itself. `--author` is the recorder, resolved as `add` resolves it; with no author it fails with `invalid`.
- **`--dry-run`** checks a **selection** and admits nothing. It is for the command line only, in a session that an adapter has bound; it is not in the tool's schema. It applies the limits of 3.5 and the secret scan to the text, finds the message the text comes from, and prints a **selection line** in place of a receipt: the session, the message, the byte range in it, the length and the SHA-256, and no part of the text (in JSON, an object `selection` with those fields). It does not choose or open a section, does not open or create any memory, checks no quota and no duplicate, and spends nothing: the store that follows does all of that. It accepts `--from-last-response`, and `--text` or `--stdin` when the text is a part of the request's own message. With no bound session, or with text that is not part of that message, it fails with `no_source`; with `--fact` it is a usage error. The skill puts the line into its one dialog question (6.2), so that what the person approves is tied to the exact bytes without their being repeated.
- **Section.** Shared unless `--personal` is given. The command prints a `destination` notice naming the section. This default is a rule for `store` alone and borrows nothing from an item's audience rules. A store never moves text from the personal section to the shared one by itself.
- **Session.** `--session`, else the environment variable `PBRAIN_SESSION` that an adapter sets; absent otherwise.

It prints a receipt and exits 0:

```
stored mem_3f9a…c2 (shared, statement, long-term, verbatim, 812 characters)
```

```json
{"id":"mem_3f9a…c2","section":"shared","kind":"statement","retention":"long-term","capture":"verbatim","text_chars":812,"said_by":"agent:architect-2","asked_by":"human:owner","received":"2026-11-26T22:40:07Z"}
```

The receipt also carries `expires` when the memory has an end, `intent` when it is learned, `source` when it is not `manual`, and `"duplicate": true` when a keyed store found its earlier self, in which case every other field is the first store's.

A refused store writes nothing, prints nothing on standard output and exits 1 with one of: `memory_not_enabled`, `invalid`, `too_large`, `secret_refused`, `quota_full` (section 7.2), `no_source` (section 6.3), `conflict`, `memory_busy`, or the personal-section errors of pbrain spec 5.

### 5.2 `pbrain recall`

```
pbrain recall [QUERY] [--task TASK] [--session ID] [--kind KIND]... [--retention TERM]
              [--id MEM_ID] [--page] [--banked]
              [--limit N] [--max-chars N] [--offset N] [--personal]
```

Recall returns memories only. It reads the store as it is at that moment, so a forgotten or expired memory is never returned. It has three modes, and a flag of one mode with another is a usage error:

| Mode | Selected by | Other flags it takes |
| --- | --- | --- |
| A page of memories | at least one of a query, `--task`, `--session` or `--retention`; `--kind` narrows any of them | `--limit`, `--max-chars`, `--offset`, `--personal`, and `--banked` with a query |
| One memory in full | `--id MEM_ID` | `--offset`, `--max-chars`, `--personal` |
| The task page | `--page --task TASK` | none |

With none of these it is a usage error.

- **Query.** Words are matched as in pbrain spec 7.1, ignoring case, with no stemming. A memory must hold at least one query word. There is no semantic recall in the first version (section 18).
- **Order.** Authored memories come before typed reports, always; a claimed source gives no advantage. With a query: by the number of distinct query words matched, then a full phrase match, then kind (`blocker`, `handoff`, a learned `statement`, any other `statement`, `outcome`), then newest `observed`, then id. Without a query: newest `observed`, then id. How often a memory was read never changes its rank.
- **Budget.** `--limit` 5 by default (1 to 20), `--max-chars` 3,000. A row shows at most 400 characters of its text, ends in "…" when cut, and always gives the full length, so a preview is never mistaken for the whole.
- **`--id`** returns one memory with every provenance field and its basis, and its text from `--offset` within `--max-chars` (default 6,000), paged as `get` pages a body (pbrain spec 6.1): `more` and `next` say how many characters are left and where to continue. "In full" means retrievable across pages: in JSON, `text` holds exactly the characters of that page, line endings and trailing spaces included, so that the pages joined in order are the stored text byte for byte. In text output the body is printed as stored. An id that is missing, expired or in the other section is `not_found`, the same answer in each case.
- **`--page`** prints the task page (section 8.1): shared memories of the task, at most 3 rows and 1,200 characters. The flag and its output exist from the first phase; what arrives in the third phase is the hook that injects the page.
- **`--banked`** adds a second, separately labelled page of banked knowledge for the same query: the results of `search`, or of `docs search` once a document folder is enrolled. That read is by words only: no model, no index, no catch-up. It uses the same section as the recall (`--personal` applies to both), at most 5 results, and its notices, such as `skipped_files`, are printed. The two pages share the one `--max-chars` limit, at least half of it kept for the banked page, and their scores are never merged. With memory not enabled, the memory page is empty with its notice and the banked page is still returned. A document search never returns a memory.
- **`--retention long-term`** with no query lists the long-term memories, newest first, for a person to prune.

A row in text:

```
Memory · said by agent:architect-2 · 2026-11-26 · long-term · asked by human:owner · task 83
  mem_3f9a…c2  [statement, verbatim, 812 characters]
  The Stop hook is the only point where the host hands us the final answer of a turn, so…
  ref: docs/plans/session-memory.md
```

In JSON each row carries the fields of 3.1 that are present, `text` (cut as above), `text_chars`, and `basis`. Personal rows returned with `--personal` are marked `personal`. With memory not enabled the page is empty and the notice `memory_not_enabled` is printed.

### 5.3 `pbrain forget`

`pbrain forget MEM_ID [--yes] [--personal]` removes one memory from the selected section. Without `--yes` it prints what would be removed and exits 1 with `confirm_required`, as `delete` does (pbrain spec 8.6). Forgetting stops every later recall and injection at once; it cannot take text back out of a model's context that already saw it, and it is not secure erasure. A second forget of the same id is `not_found`. Forgetting never touches anything in Git, including an item promoted from the memory. It works when the quota is full.

### 5.4 `pbrain promote`

Section 10.

## 6. Explicit capture

Explicit capture is the primary way a memory is made: someone says what to keep, and exactly that is kept.

### 6.1 The wording sets the term

The skill teaches agents to act on these requests, and what each means. The wording decides the kind and the term; no question is asked about either.

| Someone says | Stored as |
| --- | --- |
| "remember …", "remember that …", "take note of …", "note that …" | `statement`, long-term |
| "remember temporarily …", "remember … temporarily", "remember … for now", "keep in mind …", "remember short-term …" | `statement`, short-term |
| "learn …", "learned …", "we learned …", "I learned …", "lesson: …", a `learned:` line in a teammate's report | `statement` marked learned, long-term, always |
| "remember your last response", "take note of what you just said", "good point, let's remember that" | The previous answer, verbatim (6.3), as a `statement`, long-term |

**Who matches the wording.** The agent, guided by the skill. pbrain has no pattern on prompts and no hook reads them for this: language needs judgement, and a pattern would both miss requests and fire on text that is not one. The skill's rules:

- The phrases are cues to a speech act, not substrings to trigger on. It is a request only when it is an instruction addressed to the agent in the message being answered. The longest matching phrase decides: "remember … temporarily" is read before "remember".
- It is not a request when the words appear inside quoted or pasted content, a file, a tool's output or a log; when it is negated ("don't remember this", "no need to note that"); when it is a question ("do you remember …?"); or when it is reminiscence ("remember when we …", "remember that time …").
- A real instruction may quote its payload: "remember this exact quote: …" and "learn this exact quote: …" store the quote.
- A statement that something was learned counts when its speaker makes it directly ("we learned that X"). "We haven't learned X" and "did we learn X?" do not.
- The agent never asks about the term: the wording has settled it. When the passage is unclear it asks which passage. When the wording is clear and who is asking and where it goes are settled (6.2), it stores at once, in the same turn, before doing anything else.
- The agent stores the exact words. For "remember that X" it stores X as written; it never paraphrases and never stores its own summary under the person's name.

### 6.2 Who is asking, and where it goes

No supported host tells pbrain who typed a prompt: the same input can carry a person's keystrokes, a teammate's message, a scheduled task or a pasted forward. So the origin of a prompt is unknown until something positive says otherwise.

- **A request that arrived as a prompt.** The agent asks one question through the host's own question dialog: *for the team* or *only for me*. The question names the selection by reference, with the line that `store --dry-run` printed, and never repeats the text itself. The answer settles who is asking and where the memory goes; it does not ask about the term. *For the team* stores in the shared section, *only for me* with `--personal`. The memory records `asked_by` as that person. An answered dialog is enough to store (the owner's answer of 2026-11-26): until a host version is qualified to tell a person's answer from an automated one, `asked_by` is recorded as claimed and the memory counts in the ordinary allocation; once it is, as observed, and in the person's allocation. In either case pbrain must find the answer itself in the host's record, matching the selection, the term and the destination; an answer it cannot find counts as a dialog nobody answered. If the request already names the destination ("remember for the team that …"), the dialog still confirms it with one click, because a request to remember is not by itself permission to share.
- **A host with no question dialog, or a dialog nobody answered.** Nothing is stored: not in the shared section, not in the personal one, not with another term. Who is asking and where it goes are unresolved, and a write to the team could not be taken back by a later reply. The agent says so and gives the person the command to run themselves: `pbrain store --stdin` with the kind and term the wording already settled, and with or without `--personal` as they choose. The command does not contain the text, which the person pastes on standard input, so confidential words reach neither the reply nor a shell history. This holds whatever the request says about its destination: "remember only for me that X" is not stored anywhere on the strength of a prompt alone. A typed "yes", and a reply that only claims to come from a dialog, are prompts like any other and settle nothing.
- **A request that arrived over a supervisor's labelled team channel** (section 8.4), from a teammate. No dialog: the channel is team-visible by its label. The memory goes to the shared section with `asked_by` as that teammate.
- **An agent's own decision** to keep something, or a clear "remember" from a teammate it knows. No dialog, and no person is claimed as the requester: `asked_by` is left out or names that teammate, the section is chosen per call, and the term follows the wording or the kind's default.

An agent whose context holds private or unlabelled input (section 8.3) still stores explicitly, and must leave out of a shared memory anything that came from that input.

### 6.3 The previous answer, verbatim

`pbrain store --from-last-response` stores the immediately preceding complete, visible answer of the main agent in that session: its text, Markdown, whitespace and line endings as delivered, excluding tool output and hidden reasoning. The agent names the selection and never retypes it.

- The text comes from the host's own record of the conversation, read by the adapter at the moment of the request. That makes no new copy and needs nothing to run on an ordinary turn.
- On a host whose adapter cannot do that, `--from-last-response` fails with `no_source`, and the agent asks the person to paste the passage, which it stores with `--stdin --capture verbatim`.
- It takes an answer only from the session the adapter established for the calling process, at that session's current task and turn, and only if the task has not changed since that answer; otherwise it fails with `no_source`. No flag, environment value or path supplied by the caller can select another session's record: `--session` with it is a usage error, and a `PBRAIN_SESSION` that does not match the adapter's own context gives `no_source`. It never falls back to an older answer. An answer over the size limit is refused with `too_large`, and the person can ask for an excerpt.
- pbrain keeps no copy of answers in case someone asks later. A per-session helper that holds the latest answer in memory, for hosts with no record to read, is not in the first version (section 18).
- `said_by` is the agent that gave the answer, `asked_by` the person who asked for it to be kept, `recorded_by` the agent that ran the command; the model and effort are those of the answer's turn.

### 6.4 What one agent teaches another

Agents tell each other what they learned, and that goes into long-term memory.

- **A lesson is stored by exactly one writer.** An agent that learned something by doing, and tells nobody in that moment, stores it itself as a learned statement, in the section its source allows (below). A lesson told in a message ("we learned that X", or a `learned:` line in a report) is one teaching event, and one writer records it: the supervisor, where it records the lessons of the messages it delivers; otherwise the message's first-named receiver. The teaching agent does not also store a lesson it reports this way, and no other receiver records it.
- **The event's identity.** Each lesson in a message has a stable id of its own, given by the sender or the supervisor, that is the same across a retransmission and across every receiver of a fan-out. The writer passes it as `--event`, with the message's time as `--observed-at`. The same event stored again returns the same memory (4.3); two lessons in one message have two ids and make two memories. Whether a lesson is already stored is decided by that id and never by searching for its words: an independent message that reports the same lesson in the same words is a second witness and makes a second memory.
- **Who is who.** `said_by` is the teaching agent. On a receiver's write, `asked_by` and the recorder are that receiver, and the lesson's exact words are stored with `--capture verbatim`. On a supervisor's write, the recorder is the supervisor's neutral name and `asked_by` is absent. `said_by` is `observed` when the supervisor records it, since it knows the sender, and `claimed` when a receiver does: a sender tag is not authenticated.
- **Which section.** One rule for every writer, the teaching agent's own store included: only what came from positively team-visible input may go into shared memory. An agent whose context holds private or unlabelled input (section 8.3) that learns something from it, for example from a personal file while debugging alone, does not store that lesson in the shared section. It can keep it long-term with `--personal` where a personal section is set up and it is allowed to use it; if neither destination can be settled, it stores nothing. A lesson that arrived over the labelled team channel goes to the shared section. One that arrived any other way is treated as a prompt of unknown origin (6.2): only who sent it and where it goes are settled, never guessed from the words, and it is never silently put in another section. A receiver whose own context is paused does not write a teammate's lesson to the team by itself; a personal save stays possible.
- Neither repetition nor the long term changes a lesson's standing: it is still something said.
- **Capture rules**, in the skill and in every prompt that asks for a memory: lessons, not logs; only what this session actually showed, with something the next reader can check in `--ref`; one lesson is one memory; nothing inferred about a person; no claim that a tool does not work; and storing nothing is a good answer.

### 6.5 Offering to bank it

- **A rule or a decision.** When a person asks to remember something that reads as a standing rule or a decision ("we always run the tests with -race"), the agent stores it and, in the same reply, offers to promote it to a decision or a learning. A long-term memory has no reserved place on the task page, so a rule that must reach every session belongs in a banked item, which `prime` shows at the start of a session and which reaches other machines.
- **Lessons.** The agent does not offer promotion on every lesson. At a task handoff it offers one grouped promotion of the task's lessons into a single `learning` (section 10.2). A lesson a person taught is offered at once, like a rule.
- The offer is a sentence in the reply. Nothing is promoted without the explicit preview and confirmation of section 10.

## 7. Retention and limits

### 7.1 Short-term

A short-term memory lives seven days from `observed`, whatever its kind. In addition, short-term memories are capped by count: the newest 32 per task; for a memory with no task, the newest 32 per enrolled recording agent; and for a manual memory with neither, one shared "unassigned" bucket of 32 per section. Whichever limit is reached first drops the oldest, by `received` and then id. A claimed author name or an invented tag never opens a fresh budget. A dropped memory leaves nothing readable behind, and a replay of it is refused (4.3). There is a hard ceiling of 10,000 short-term memories per section.

The number 32 is a starting value. The pilot (section 17.3) counts how many memories a task really collects in a week, and the number is set from that.

### 7.2 Long-term

A long-term memory stays until someone forgets it or until the date its requester chose with `--until`. It is never dropped to make room, never removed by promotion, and untouched by a cache purge. Each section holds at most 500 long-term memories and 16 MiB of their text, split in two allocations (the owner's answer of 2026-11-26):

- **The person's allocation:** 200 memories and 6 MiB, for memories that a person asked for. It counts only a requester the adapter observed as a person (3.4). "A person" here means a human being: the factory's operator agent is an agent, and a memory it stores by its own choice does not count.
- **The ordinary allocation:** 300 memories and 10 MiB, for everything else: what agents learned or chose to keep, and any memory whose `asked_by` is only claimed.

The allocation is decided once, when the memory is admitted, by who asked for it, whoever said the text and whoever recorded it: an answer that an agent gave and stored at a person's observed request is in the person's allocation. A duplicate store returns its original receipt and is not counted again. When the allocation a store belongs to is full in either measure, that store is refused with `quota_full`; the message says how to list the long-term memories (`pbrain recall --retention long-term`), and `forget` still works. A full ordinary allocation never refuses a request that was observed to come from a person; a request whose `asked_by` is only claimed is ordinary and can meet `quota_full`. A short-term store is never refused for this reason.

### 7.3 What neither limit touches

Nothing in Git is ever expired, dropped or deleted by memory: not a managed item, not a document, not an item promoted from a memory.

## 8. Long sessions

A session can run for days and hold many tasks. Nothing here happens "at the end of the session".

### 8.1 The task page

The task page is one frozen page of the task's memories: at most three rows and 1,200 characters, labels and refs included, in recall's order without a query. It is what `pbrain recall --task T --page` prints. Each row is quoted as an attributed lead: the agent checks the cited source before treating the claim as established, and never takes a row as an instruction. A long-term memory has no reserved place on it. Memory only: no document or item appears on it in the first version (section 17.3).

It is put in front of an agent:

- at session start, on resume and on fork;
- after every context compaction;
- when the agent's task changes.

It is read fresh each time, once per task and context, so a forgotten or expired memory never comes back and a teammate's new memory arrives. Without a task there is no page: the skill tells the agent to recall with words from its work before a non-trivial task, as it tells it to search. Who receives a page is checked each time, at a start, a fork, a resume and a compaction alike. A page is returned only to a session whose host called the hook a person installed for that project (section 9.3), or that a supervisor set up, and only after the brain has been opened with its normal guard for that session's folder. A task token inherited through the environment selects which page, and grants nothing. The injected page holds shared memories only: no personal row is ever injected in the first version, so a forked or unknown session that inherits a task receives nothing private. Personal memories are read with an explicit `recall --personal`.

### 8.2 What runs on an ordinary turn

Nothing in this design asks the model anything on an ordinary turn, and pbrain itself configures nothing to run at the end of a turn (section 9.2). The page is injected by events that fire at a start, a resume or a compaction, not per turn.

### 8.3 The checkpoint, and what pauses it

A **checkpoint** is one short memory (kind `handoff`, `outcome` or `blocker`, capture `checkpoint`, at most 2,048 bytes) that an agent is asked for when it hands off a task. It is the one write into team memory that happens without anyone choosing its destination at that moment, so it needs a supervisor that can vouch for the agent's whole context. pbrain alone never prompts for one.

- **One trigger.** A task handoff reported by the supervisor, or an explicit checkpoint request. It is serviced once, at a suitable turn boundary. Idle may coincide with it and never supplies it. There is no timer and no prompt per turn.
- **The prompt** asks for at most one memory: what was verified, what remains unresolved, the next action, and what the next agent can check. Storing nothing is a valid answer.
- **Eligibility.** The supervisor allows the checkpoint only for an agent whose whole context it can vouch for as team-visible: a fresh context with no earlier conversation, compacted summary or private tools and files inherited, fed only by input the supervisor labelled team-visible before delivering it.
- **The pause.** If such an agent is handed something private, or something nobody labelled, the supervisor first switches off automatic checkpoints for that agent's current context, because the agent might repeat the private text in its next summary. That is all that is switched off. The agent's task, explicit `store`, personal saves, recall and the task page keep working.
- **How long.** Until that agent starts from a new, clean context with a fresh grant. A new task token or a compaction does not lift it, because the private text can still be in the context or its summary. The same pane can host the clean context: reusing the pane proves nothing, the clean context does.
- **Who is affected.** Only the agent that saw the input. Typing into one agent's pane changes nothing for any other agent.

In the owner's factory: the owner types a confidential note into the operator's pane. The operator keeps working and can save explicitly, but its next automatic checkpoint is withheld. The operator then sends the members their assignments over the bus; those are labelled team-visible, so the members keep their automatic checkpoints through sessions of any length. The operator, and any pane that mixes typed and labelled input, stays on explicit saves; the skill reminds an agent to store a handoff when it reports a task, and nothing is written unattended. There is no setting that makes whatever is typed into a pane count as shared.

### 8.4 What pbrain provides and what a supervisor provides

| pbrain | A supervisor (the factory) |
| --- | --- |
| `store`, `recall`, `forget`, `promote`; the task page; the session-start and compaction hooks (section 9) | The labelled team channel, and the label on every delivery before it is delivered |
| Admission into memory: size, secret scan, section, quota, replay | The task token of each agent, and the page injected with a new assignment (it calls `recall --page`) |
| `capture: checkpoint` and its 2,048-byte limit | The checkpoint trigger, the grant, eligibility and the pause |
| The same answer to every caller: no caller is trusted more for the `--source` it claims | Writing a restricted worker's checkpoint for it: such a worker has no write access to the state folder, so it cannot go round the pause |
| | A neutral, enrolled name for the recorder of an automatic write, never a person's Git identity |

A supervisor's work is built and tracked in its own repository. Normal agent work goes on when memory is off, pbrain is missing, or memory refuses a write.

## 9. Hosts

### 9.1 One command for every host

`pbrain hook EVENT --harness NAME` reads the host's event as JSON on standard input and answers in the form that host expects. It is not a tool and is not in the catalog. In the first version it handles two events:

| Event | When the host fires it | What pbrain does |
| --- | --- | --- |
| `session-start` | A session starts, resumes, forks, or comes back from a compaction | Notes the session's id and model for provenance; if memory is enabled and the session has a task (the environment variable `PBRAIN_TASK`), returns the task page as added context. Otherwise returns nothing |
| `pre-compact` | The host is about to compact the context | Returns one instruction for the summary: keep the task token, and expect the task page again after compaction |

A hook never fails a session: on any error, or after its time limit, it returns nothing and the session goes on. It makes no network request and loads no model.

### 9.2 Plugin or configuration

| Harness | How it is wired | First version |
| --- | --- | --- |
| Claude Code | Configuration only: command hooks in a settings file | Qualified first |
| Codex | Configuration only: `.codex/hooks.json` or `[hooks]` in `config.toml` | Candidate |
| Antigravity CLI | Configuration only: `.agents/hooks.json` in the workspace | Candidate |
| pi, omp | One small generated extension file that runs the same command | Candidate |
| OpenCode | One small generated plugin file, the same way | Candidate |

A candidate is known from its documentation only. A documented surface is not a working one: a host is supported when its deployed version has been shown to deliver the task page after a start and after a compaction, and to supply the previous answer, on a real session. On every host, with no hook at all, the four memory commands and the skill work, and "remember that …" works.

### 9.3 `pbrain hooks install`

`pbrain hooks install --harness NAME [--shared] [--print] [--yes]`, `pbrain hooks uninstall --harness NAME [--shared] [--yes]` and `pbrain hooks status` set an adapter up for one project. They are commands for a person, not tools.

- `install` without `--yes` shows the exact change and exits 1 with `confirm_required`; with `--yes` it writes. `--print` shows it and writes nothing.
- By default it writes the person's own, untracked settings file for the project (`.claude/settings.local.json` for Claude Code); `--shared` writes the project's tracked one. It keeps every setting already there and leaves the host's own trust prompt in place.
- `uninstall` removes only pbrain's own entries, and only if they are unchanged: a file nobody touched since the install comes back byte for byte, an unrelated edit made since is kept, and a pbrain entry the person edited is reported as a `conflict` and left alone.
- Nothing is installed for the whole machine, and no skill edits a host's configuration by itself.

### 9.4 Cost

- `pbrain hook` adds one process start per event it is configured for. Neither of its events fires on an ordinary turn.
- Budget for the session-start hook: the task page within 100 ms at the 95th percentile, warm, with 10,000 memories in the section, on a named quiet machine.
- Budget for a supervisor's end-of-turn check, where a checkpoint is built on a host's Stop event: an ordinary end of turn adds at most 25 ms at the 95th percentile and 75 ms at the 99th over the same host with the adapter off, with no model call, no prompt and no durable write on that path, and a hook that overruns is abandoned.
- Both budgets are requirements to be measured (section 17.2), not measurements. The one figure measured so far is the start of the pbrain binary: 3.5 ms at the median over 200 runs on the team's Linux machine.

## 10. Promotion: banking a memory

### 10.1 One memory into one item

```
pbrain promote MEM_ID --kind learning|decision --title TITLE [--decided ACTOR]
               [--text TEXT | --body-file FILE] [--quote MEM_ID]... [--max-chars N] [--personal]
pbrain promote MEM_ID... --kind document (--to PATH --title TITLE | --update PATH)
               [--text TEXT | --body-file FILE] [--quote MEM_ID]... [--max-chars N]
pbrain promote MEM_ID... --confirm TOKEN [--personal]
pbrain promote --show TOKEN [--offset N] [--max-chars N] [--personal]
```

Promotion has two steps, and only the second writes an item. The first writes only a small, bounded record beside the memory: the frozen draft and its token.

- **Preview.** pbrain builds the complete Markdown of the new item once, front matter included, and prints it with its target path. The body is the memory's text, or `--text` or `--body-file` when the author edited it, followed by a **Sources** part (10.3). A learning starts provisional; a decision needs `--decided` and follows the existing rules for decisions (pbrain spec 3 and 8.1). The preview returns a token that is valid for ten minutes, the target path, the draft's length and its SHA-256, and the draft itself within `--max-chars` (default 6,000). A draft longer than that is not cut silently: `more` says how much is left, and `pbrain promote --show TOKEN [--offset N] [--max-chars N]` prints the rest of the same frozen bytes without changing them or the token. A draft holds its sources' text, so `--show` is checked like a read of them: before every page it checks the section the caller selected, that the token is unspent and inside its ten minutes, and that every source is still there and visible. If a source was forgotten, has expired or is in the other section, it prints no bytes, no digest and no partial draft, and answers `not_found`. This does not touch the acknowledgement of an item already published, nor the item in Git. A new preview of the same memory cancels the older token.
- **Confirm.** `--confirm TOKEN` publishes exactly the previewed bytes into the checkout the command runs in, through the same guarded writer as `add`. No flag can change anything at this step. If the author, the section, the audience rules or the checkout changed since the preview, the token is refused and a new preview is needed.
- **Only authored memories.** A memory with capture `authored`, `verbatim` or `checkpoint` can be promoted. A typed report is refused with `usage` (exit 2) before any preview, token or write, whatever title, text, decider or token is supplied. A memory that is missing, expired or in the other section is `not_found`.
- **Sections.** A shared memory promotes to a shared item, a personal memory to a personal item. Nothing crosses.
- **The original stays.** Promotion publishes a separate item, which may be an edited draft. The memory is unchanged: a short-term one expires on schedule, a long-term one stays until it is forgotten.
- **Spend before publish.** The token is marked spent, and that mark is made durable, before anything is published; only the call that made the mark may publish.
- **What the token is.** It confirms bytes. It grants no standing and is not a human approval: an agent may preview and confirm in its review checkout, and the team accepts the item through its ordinary Git review. A hook never confirms.

**Under failure.** The invariant is at most one item per token.

| What happens | Result |
| --- | --- |
| Two confirms at once, no fault | Exactly one item; the other call is told it was already published |
| The token's spending could not be made durable (a refused sync of the state file or its folder) | No item, even if the spent mark is already visible. The call fails with `io`, saying the token may have been consumed. A retry cannot publish from that visible mark |
| A crash after the token was durably spent but before the item was written | No item. A retry reports `conflict` with an indeterminate outcome and writes nothing. The author looks at Git and, if an item is wanted, makes a new preview or uses `add` |
| The reply was lost after the item was written; the same confirm again | The existing item is acknowledged if its bytes match; nothing is written twice, even if the memory was forgotten in between |
| The source was forgotten or expired before the first publication | The token is refused; no item |

### 10.2 Several memories into one

`pbrain promote MEM_ID MEM_ID … --kind … --title … (--text TEXT | --body-file FILE)` turns up to 16 authored memories into a single item. This is how a task's lessons become one reviewed learning, and how what several agents said becomes one decision.

- The author writes the consolidated text; with more than one memory, `--text` or `--body-file` is required. pbrain does not summarize.
- It is one proposal: one section, one fixed set of sources, one preview, one token, one item. `--confirm` takes the same ids.
- Before the first publication every source is checked again, and the whole proposal is refused if any source was forgotten, has expired, is a typed report, or is personal while the target is shared. A proposal made wholly of personal memories into a personal item is allowed.
- The failure table of 10.1 applies to the group as to one memory. Every original stays.

- **Allowed combinations.** A new item: ids, `--kind learning|decision`, `--title`, and optionally `--decided`, a body, `--quote`, `--personal` and `--max-chars`. A new document: ids, `--kind document`, `--to`, `--title`, and optionally a body, `--quote` and `--max-chars`. Document folders are shared, so `--personal` with `--kind document` is a usage error, for a new document and for a patch. A review patch: ids, `--kind document`, `--update`, exactly one complete proposed body (`--text` or `--body-file` on the command line, `text` in a tool call) and optionally `--quote` and `--max-chars`, with no title, no token and no confirm. Confirm: the same ids and `--confirm`, and nothing else but `--personal`. Show: `--show TOKEN`, with `--offset`, `--max-chars` and `--personal` and no ids. Anything else is a usage error.
- **As a tool.** `pbrain_promote` takes `ids` as an array of 1 to 16 memory ids, required for a preview and a confirm and not allowed with `show`, and `quote` as an array of ids from that set; the body is the `text` field, and `--body-file` is for the command line only. On the command line the ids are repeated operands and `--quote` is repeatable. `pbrain_docs_inventory` takes `folders` as an array in the same way.

### 10.3 Sources a teammate can use

The banked item must make sense on a clone that has none of the memories. So pbrain appends a **Sources** part to the body, one entry per memory: who said it, who asked, when, how it was captured, and its `ref`. A machine-local memory id is not a citation and is not printed there.

- A source is cited by its `ref` only if the ref is **portable**, meaning something a clone can resolve: an `http` or `https` URL; a path from the project root that names a file Git tracks; a commit id the repository holds; or the id of a managed item in the same section. A memory id, an absolute path, a path outside the project and free text are not portable.
- A source without a portable ref must be quoted: `--quote MEM_ID` puts the whole stored text of that memory into the Sources part, where the reviewer sees it. There is no selector for a part of it in the first version; an author who wants less writes the excerpt into the body and gives the source a portable ref.
- A source with neither is refused with `invalid`, naming the id.

### 10.4 A document as the target

Available with the documents phase (section 17.1, phase 5).

- **A new document.** `--kind document --to PATH` with `--title` and a body. `PATH` is a path from the project root, under an enrolled document folder, that does not exist. Preview and confirm work as in 10.1; confirm creates the file and fails with `conflict` if it has appeared meanwhile. pbrain creates this file, so it writes a header with the title, the creation time, the recorder and the Sources part.
- **A change to an existing document.** `--kind document --update PATH` with the complete proposed text as its one body, given as `--text` or `--body-file` on the command line and as `text` in a tool call, and `--quote` for any source without a portable ref. pbrain prints a patch from the document's current bytes to the proposed ones, with the digest of the bytes it read, and writes nothing. The proposed text gets the same Sources part appended as a new document does, under the same rule of 10.3, so the patch carries its citations and the author may move them. It is a patch for review and makes no promise about the file it is later applied to: plain `git apply` applies hunks even when the document has changed elsewhere, so whoever applies it compares the digest first if the exact base matters. pbrain never replaces a file someone else may be editing, and never writes a header into a document it did not create.
- The agent searches the documents first and proposes a change to an existing one before creating another.
- No promotion installs a skill, rewrites a host's instructions or deletes a reviewed file.

### 10.5 Across machines

Each machine has its own short-term and long-term memories. Git carries what the team shares: two machines can each propose cited contributions to the same item or document, and a third finds it after a pull with an empty memory. pbrain copies no unreviewed memory between machines.

## 11. Documents

### 11.1 Naming the folders

`pbrain.toml` stays where it is, in the brain folder, and still marks the brain. It gains one table:

```toml
[docs]
folders = ["docs", "specs"]
```

- Each entry is a path from the project root, with forward slashes. An absolute path, a path with `..`, an empty entry, more than 16 entries, the same folder twice, and one listed folder inside another are `invalid_config`.
- With no `[docs]` table there are no documents. `docs search`, `docs get ID` and `docs list` then answer from managed items alone. `docs sync` and `docs check` have nothing to do: they succeed, print nothing to build or report, and give a `docs_not_configured` notice. `docs inventory` works before any folder is enrolled; that is its purpose.
- A brain outside a Git work tree has no project root: a `[docs]` table there makes the `docs` commands fail with `git_required`, and every other command ignores it.
- **A new project.** When `pbrain init` creates a new brain at the default place, `docs/brain`, it writes `folders = ["docs"]` and prints how many files that makes searchable. `init --path` anywhere else, and an `init` that completes a brain that already exists, write no table.
- **An existing brain** changes nothing until a person adds the table. `pbrain docs inventory FOLDER…` shows first what would become searchable, and changes nothing. pbrain never edits `pbrain.toml` after `init`.
- Document folders are shared. The personal section holds items only, as today.

### 11.2 Which files are documents

A document is a regular file whose name ends in `.md`, under a document folder, that is none of these:

- inside the managed area. The default brain `docs/brain` and its personal section `docs/brain/personal` lie inside the default folder `docs/`; the managed area is pruned whole from the folder's walk, with everything in it, before anything is read or counted, and its files are read only as items;
- inside a folder whose name starts with a dot;
- matched by a Git ignore rule, tracked or not;
- a symbolic link, or reached through one;
- over 4 MiB, or not valid UTF-8 (both reported by `docs check`).

A document folder that reaches a personal path in any other way, directly or through a link, is refused with `personal_exposed`. A new file dropped into a folder is a document from the next command on; there is nothing to add.

### 11.3 The folder decides what a file is

A file in the managed area is an item under today's rules, front matter and all. A file under a document folder is a document, whatever its header says.

| The file's header | What pbrain does |
| --- | --- |
| None, or a well-formed header with no usable `title` | The title is the first Markdown heading, else the file name without `.md` |
| Well-formed YAML between `---` lines | `title`, if it is a single-line string, becomes the title, shown and searched. Nothing else is read. The rest of the header is not searched; the body is. The body is what follows the header's closing line and the one empty line after it, if there is one |
| Broken or unclosed | The whole file is plain text from its first line. `docs check` warns |

A document that carries `status: verified`, `decided:`, `replaces:`, `kind:` or `author:` gains nothing from them: it has no status, no standing, no author for pbrain, no place in the replacement graph, and never appears in `prime`. Line numbers always count from the top of the file, header included. pbrain never writes into a document it did not create.

### 11.4 Addresses

A document is addressed by its path from the project root: `docs/plans/tool-schema.md`. That is what results print as `path` and what an editor opens. Two files named `README.md` in different folders are two documents. A rename changes the address; a citation that must last belongs in a managed item or names a Git revision. Two documents whose paths differ only by case are both left out of results and reported by `docs check`.

An address and an item id are never told apart by their look: an old item may have an id such as `report.md`. `docs get --path PATH` reads a document and `docs get ID` reads an item. Every `path` and every `id` that a `docs` command returns opens with the matching form.

### 11.5 The `docs` commands

All but `docs add` are new tools: `pbrain_docs_search`, `pbrain_docs_get`, `pbrain_docs_list`, `pbrain_docs_sync`, `pbrain_docs_check`, `pbrain_docs_inventory`.

**`pbrain docs search QUERY`** searches managed items and documents together and labels each result with its class.

- Query, words, clauses, "no fallback", snippets, location lines, occurrences, `--context`, `--limit` (5), `--max-chars` (3,000), `--offset`, `--personal` and `--all` are those of pbrain spec 7.1. A document has a title and a body and no tags.
- **Order** (the owner's answer 6a). Results are sorted by match class first, exactly as in pbrain spec 7.1: every clause in the title; else in the title or the tags; else any other match. Inside one match class the items come first, in today's order (recency band, standing, newest, id), then the documents, by keyword score (the BM25 formula of pbrain spec 7.1, over title and body), highest first, then by path. The rule that moves the first decision in force to the top is unchanged and looks at items only.
- **Filters that only items have.** `--kind`, `--tag`, `--status`, `--since` and `--until` leave documents out, since a document has none of these; the closing line then counts the matching documents left out, in JSON `documents_filtered`. `--class item|document` keeps one class.
- **`--semantic`** adds, after the exact results, items and documents found by meaning, as in pbrain spec 7.1, with the same per-model cut-off, so a question nothing answers still returns nothing. It needs the index of each class it searches: the item index (`pbrain sync`) and the document index (`pbrain docs sync`). A search narrowed to one class, by `--class` or by a filter only items have, needs only that class's index. For a side that is not built it returns what it has with a `semantic_not_ready` notice naming that side.
- **Output.** In JSON every result has `"class": "item"` or `"class": "document"`. An item has the fields `search` gives it. A document has `path`, `title`, the location fields of pbrain spec 7.1 and no `id`, `kind`, `status` or date. In text an item's block is `search`'s, with `Item · <kind> · <status>` ahead of it; a document's first line is `Document · <path>`.

**`pbrain docs get --path PATH`** prints one document: its path, title and sizes, then its body from `--offset` within `--max-chars` (6,000), with `--outline` as in pbrain spec 7.2. **`pbrain docs get ID`** answers as `get` does. Giving both, or neither, is a usage error.

**`pbrain docs list`** lists items, in `list`'s order, then documents by path, each labelled; `--class` and `--folder FOLDER` narrow it. `--limit` 20, `--max-chars` 2,000.

**`pbrain docs sync`** builds the document index for semantic search, with the flags of `sync` (pbrain spec 9.4), including `--status`. It embeds only passages whose text has no vector yet. It never touches the item index, and a finished `docs sync` never makes an unbuilt item index look ready: each side reports its own readiness, counts, notices and errors.

**`pbrain docs check`** reports document problems, ordered and paged as `check`'s findings, and exits 1 if any is an error. Plain `check` keeps reporting items only, with today's findings.

| Code | Severity | Meaning |
| --- | --- | --- |
| `doc_unreadable` | error | The file cannot be read |
| `doc_not_utf8` | error | Not valid UTF-8. Left out of results |
| `doc_too_large` | error | Over 4 MiB. Left out of results |
| `doc_path_collision` | error | Two paths differ only by case. Both left out |
| `doc_bad_header` | warning | A header that does not parse or is not closed; the file is read as plain text |
| `doc_folder_missing` | warning | A folder named in `[docs]` does not exist |
| `secret_suspected` | warning | The document matches a secret pattern; the finding names the path and line, never the value |

A missing header is not a finding.

**`pbrain docs inventory FOLDER…`** shows what enrolling each folder would make searchable, for a folder that is or is not yet in `[docs]`: the number of documents and their total size, and the number of files left out for each of these reasons: ignored by Git, a link, too large, not UTF-8; with the first paths of each under `--limit`. It changes nothing. It walks exactly as section 11.2 does, so it never enters what that walk prunes: a managed area inside the folder is reported as one skipped root by its configured path, with no count, size or path from inside it, and nothing about a personal section is read, counted or named. A folder that would reach a personal path in any other way is refused with `personal_exposed` and no detail. Its output is the same with and without any private file.

**`pbrain docs add`** is another spelling of `add`: it writes a managed item. The catalog lists that operation once, as `pbrain_add`.

### 11.6 Found at once, and the cost of meaning

"Drop a file in and it is found" means `docs search` finds it by its words at once, with no index and no model, and by meaning after the next `docs sync` or the bounded catch-up of pbrain spec 7.1. Lexical search reads the files on every call, so its time grows with the folders. Semantic search needs the default model to embed every passage once: on processors alone that is about 1.3 passages a second (measured for v0.5.0), so a wide folder is hours of first indexing without a GPU. For scale, this repository's own `docs/` and `specs/` hold 206 Markdown files and 2.7 MB. That is why an existing brain enrols a folder deliberately, after seeing its inventory.

### 11.7 Indexing on the processor: a warning, the reason and an estimate

Building an index on the processor works and is slow, and a person or an agent should know that before waiting on it. So when `sync` or `docs sync` is about to embed on the processor with a model that can run on a GPU, the `cpu_fallback` notice of the pbrain spec (9.4) becomes a clear warning. It is still a notice on standard error and changes nothing about the run: the run goes on, ends, fails or is cancelled exactly as it would have, with the same exit code, and a run that is stopped is resumed by the next `sync`.

The warning has three parts:

1. **Why the GPU is not used**, one of the five reasons of today:

| Reason in JSON | What the text says |
| --- | --- |
| `asked` | The processor was asked for, naming where: `--device cpu` or `PBRAIN_DEVICE=cpu` |
| `unsupported` | This build of pbrain has no GPU support for the model |
| `absent` | No usable GPU was found |
| `failed` | The GPU could not be used: looking for one failed, or one was found and could not be started |
| `memory` | The GPU has too little free memory, at the start or, under `auto`, after it ran out during the run |

2. **How much is pending:** the number of distinct passages that the run, at the moment of the warning, plans to give the model for this index. It is worked out by the same rules the build itself uses to decide what to embed: a passage whose vector the build would take from the index or from the cache is not counted, a cached vector the build would reject as damaged is, and a passage that occurs several times is counted once. Planning ahead embeds nothing. The count for the shared index never looks at anything of the personal section. On an unchanged tree with no other pbrain at work the model is asked for exactly that many passages; when another checkout fills the shared cache meanwhile, the run does less than it announced.
3. **About how long that takes:** the count divided by the model's measured processor rate. Each model of a release carries one such figure, in passages a second: the lowest that was measured for it on four cores (pbrain spec 16). For this release that is 1.11 for the default model and 0.902 for Google's pack. The estimate is rounded up to whole minutes, is given as "under a minute" below one, and is labelled "about", with the rate beside it: it comes from another machine, and a faster or slower processor or longer passages move it. A model with no measured figure gets the count and the words "no estimate"; pbrain never makes a rate up.

```
cpu_fallback: warning: shared: indexing on the CPU, which is slow: this pbrain has no GPU support for embeddinggemma-2-q8-0; 4812 passages to embed, about 1 h 13 min at 1.11 passages a second
```

With `--json` the notice keeps `code` and `message` and gains `section`, `reason`, `passages`, and, when there is an estimate, `passages_per_second` and `estimated_seconds` (here 4336); with no estimate both of those are absent. A caller that uses pbrain through its tool catalog receives the same notice with the same fields as one that reads standard error.

**When it is printed.** Once for each index the run embeds into on the processor, naming the section as progress notices do: the shared items, the personal items, the documents. There are two moments. Where the processor is the device from the start, it is printed immediately before the first passage of that index is given to the model, and never before it is certain that there is one. Where an automatic choice moves from a GPU to the processor during a run, the model finishes the group of passages it was working on, on the processor, and the warning is printed when that group is done, after `embedding_device`; it counts what is planned after that group, which can be nothing, and then it says so and gives no estimate. A group is at most one window of passages, so this warning can come some minutes after the GPU gave out. A run that embeds nothing prints nothing, and so does a model that has no GPU path.

**A search stays silent**, by design. `search --semantic` and `docs search --semantic` embed their query on the processor in every build, and their bounded catch-up embeds a few passages there; neither prints `cpu_fallback`.

## 12. Where state lives

The owner's decisions of 2026-11-26: pbrain keeps **no folder of its own in the repository**, for the shared brain or for the personal section; and its databases are ordinary SQLite files on disk, written in place. This section replaces the `.pbrain/` folders of the pbrain spec (its sections 2.1, 5, 8.4, 8.6 and 9.4) wherever they are named.

### 12.1 What is left in the working tree

Only what people and Git own: `pbrain.toml`, the items, the documents, and for one person `personal.toml` and the personal section's items. There is no `.pbrain/` in the brain and none in the personal section: no lock file, no originals, no index. The rule is about what pbrain keeps while it runs. Two files in the tree are still written on request, as today and as section 9.3 says: `init` writes `.gitignore`, and `hooks install` edits the host's settings file for the project.

### 12.2 Two per-user folders

| Folder | Holds | If it is deleted |
| --- | --- | --- |
| **The cache**: `PBRAIN_CACHE`, else `pbrain` under the user's cache folder (`~/.cache` on Linux, `~/Library/Caches` on macOS) | What can be rebuilt: model files, vectors, the semantic indexes | Nothing is lost; `pbrain sync` rebuilds |
| **The state folder**: `PBRAIN_STATE`, else `pbrain` under `$XDG_STATE_HOME` when that is an absolute path, else `~/.local/state/pbrain` on Linux and `~/Library/Application Support/pbrain` on macOS | What cannot be rebuilt: memories, the originals of imports; and pbrain's small records (enrolment, host sessions, installed hooks, migration receipts) | Memories and originals are gone |

Both folders are owner-only, and neither is ever walked for documents.

**Nothing in them can be used as input, ever.** Everything pbrain newly keeps in either folder (memories, originals, records, indexes, personal vectors) sits below a folder named `.pbrain` inside it. `import` and `--body-file` already refuse any input whose resolved path has a `.pbrain` component, wherever it is, and that rule stays: so a personal original is refused as a shared input even after `PBRAIN_STATE` or `PBRAIN_CACHE` has been pointed somewhere else and the old folder is no longer "the" state folder. In addition, an input inside either folder as configured now is refused. No `.pbrain` folder is created in a working tree. The model files and the vectors of shared passages are not private and keep their present places in the cache.

**The two folders must be apart.** After resolving links, the cache and the state folder must be two different folders, neither inside the other, and both outside every Git working tree, every brain and every personal section. Otherwise deleting "the cache" could delete the only copy of the originals and the memories. A configuration that breaks this, by `PBRAIN_CACHE` or `PBRAIN_STATE` or by a link, makes every command that needs either folder fail with `invalid_config`, naming the variable, before it creates, moves or deletes anything; commands that read items only are not affected. This is stricter than today, where a cache inside a working tree was accepted if Git ignored it: such a setting is now reported, nothing new is written there, and the person moves the folder or unsets the variable.

**Changing where the state folder is** is something a person does with nothing running. pbrain does not move data between two state folders: memories and originals kept under the old one are not found under the new one until the person moves the folder. Model files and shared vectors keep their present places inside the cache, so nothing is downloaded or embedded again because of the move.

### 12.3 What is keyed by what

| State | Folder | Key | Shared between worktrees of one repository? |
| --- | --- | --- | --- |
| Model files; vectors of shared passages | cache | content | Yes, and between projects: identical text embeds once |
| The shared item index, and the document index | cache | project and checkout | No: two worktrees hold different bytes at one path, even at one commit |
| Originals of shared imports | state | project and checkout | No: each checkout keeps the originals its own items refer to, as today |
| Shared memory | state | project | Yes |
| The personal index, personal vectors, originals of personal imports, personal memory | cache (index, vectors) and state (originals, memory), each in a folder of the personal section | the personal section | With every checkout that uses that personal section, as today |

- **Project:** the repository's Git common directory and the brain's path in the work tree; outside Git, the brain's resolved path. **Checkout:** the work tree's resolved root. **Personal section:** its resolved root.
- **A path that was reused is a new place.** Each key is bound to the identity of the folder it was made for, as memory's enrolment is (4.2). A worktree deleted and created again at the same path, a different repository put at an old path, and a personal folder replaced by another are each new: they start with no originals, no index and no completed move, and never see their predecessor's. The predecessor's data is left where it is.
- **Two clones of one repository on one machine** have two common directories, so two project keys: they share nothing but models and vectors, and each has its own default personal section. The same remote and the same commit make no difference. Only if a person points both clones at the very same personal folder do they share that personal section's data, as one personal section always has.
- A personal section's data is never in a folder that a shared call opens, and personal vectors are never reused by a shared index or the other way round. This replaces the pbrain spec's rule that personal data lives in the personal section's own `.pbrain/` and that the per-user cache holds nothing personal: it now holds the personal index and vectors, in their own folder.
- A worktree that was deleted leaves its index and originals behind. pbrain never deletes another checkout's data by itself; section 20 has the open point.

### 12.4 Moving an existing brain

A brain or a personal section that still has a `.pbrain/` folder is moved once, per checkout and per section, by the first command that would write there (`add`, `import`, `verify`, `delete`, `sync`, `audience`, `init`), and never by a read.

1. Everything in the move happens during the section's turn of section 12.5, the one every writer of this version takes: finding the old folder, looking for the receipt, copying, cutting over and cleaning up. The old version's lock file is not taken and not recreated: old and new pbrain are not run together (below), and the clean-up removes it with the rest. Under those locks every original is copied to the state folder, read back and compared with its digest and length, and it and every folder newly created for it are synced to disk.
2. **The cutover is the receipt.** Only when every original has been copied, verified and synced is a receipt written in the state folder and itself synced. If any copy, comparison or sync fails, including the receipt's, nothing old has been removed, the command that triggered the move fails with `io`, and the next writing command tries again.
3. After the receipt, the old originals, the old index and the old lock file are removed, and the `.pbrain/` folder if it is then empty. From the receipt on, originals are read from the state folder only. A crash during this clean-up loses nothing: the next writing command sees the receipt and finishes the clean-up, and never copies an old file again, so an original deleted since is not brought back. A file in the old folder that is not pbrain's is left, with the folder; the receipt says the move is done, so it is not tried again on every write.
4. The index is not copied: `search --semantic` gives its lexical results with `semantic_not_ready` until `pbrain sync` has run once. That sync reuses every vector the cache holds and can read for the same text and profile, and embeds only what is missing. A personal index is rebuilt from the personal section's own vectors and embeds what it lacks; it never borrows a shared vector.
5. The command prints a `state_moved` notice with the number of originals and the new place.

Until the receipt, a read finds an original in the old folder; a read that runs while the move is in progress finds it in one place or the other, never in neither: one that loses the old file at the cutover looks again in the state folder. Two first writers at once move once. The same holds for a personal section that several checkouts use. A personal section is moved only by a command that is allowed to write it. The move itself writes the state folder, so the first writing command on a brain that still has a `.pbrain/` folder needs that folder writable, whatever the command.

An older pbrain does not know the new places or the new lock. Running an old and a new pbrain against one checkout, or against one personal section from different checkouts, at the same time is not supported, and the release note says so. `init` keeps writing the `.pbrain/` line into `.gitignore` beside `personal/` and `personal.toml`, so that a folder an old version creates is never committed.

### 12.5 Taking turns, without lock files

By the owner's decision of 2026-11-27 pbrain keeps **no lock files**. Where its processes must take turns, they do so through SQLite, with a write transaction on a small database in the state folder: one writer is let in at a time, the next waits up to its time limit and then fails with a clear message, and if the writer is killed the operating system releases its hold at once and nothing half-done remains in the database.

**What needs turns at all.** SQLite already takes care of its own databases: two agents storing a memory, or a search during a `sync`, need nothing more. What is left is the Markdown, which is not in a database:

| What | Why creating a file exclusively and replacing it by rename are not enough |
| --- | --- |
| A new item's id | The same id with other capitals, or in another kind's folder, is another file name and still a duplicate |
| An import | Whether these bytes are already imported is a look across the section, then a write; the original and the item belong together |
| `replaces` and standing | The replaced item must exist and be current and no cycle may form: a look across the section, then a write |
| `verify` | Read, change, rename back: two at once lose one change |
| `delete` | What becomes current again, and whether the original may go, depend on every other item of the section |
| `audience` | Read `personal.toml`, change it, write it |
| The move of an old `.pbrain/` | Copy, verify, cut over, clean up |
| A promotion | A memory row and a Markdown file that belong together |

**How.** Each section, which is the managed area of one checkout or one personal section, has one small database in the state folder that holds nothing but the turn. A command that writes that section's Markdown opens it, begins a write transaction, looks and writes, and ends the transaction; nothing is ever stored in it, so a turn is never left behind as a row that a crash could strand. A promotion takes the destination section's turn first and keeps it to the end.

- Two worktrees have two sections and do not wait for each other. Every checkout or clone that uses one personal section shares its database and waits on it, as today.
- A shared write never touches a personal section's database.
- A writer waits up to 30 seconds or its caller's time limit, whichever is less, and then fails with `io` having changed nothing.
- **What pbrain records about itself** (which memory belongs to which project, which folder has which id, what `hooks install` inserted, that an old folder was moved) is kept as rows in one small database of the state folder, in place of the marker and record files of earlier drafts. `hooks install` takes its turn per settings file, as an item write does per section.
- **A build of an index** can run for hours and must not keep item writers out, so each index has its own small database for turns: a build holds it for its length, a search's catch-up tries once and gives up, and a search holds it for reading while it reads the index. The same holds for a model download.

**What this cannot cover**, and nothing here could: a person or Git changing the Markdown while pbrain writes, as today; a network file system, which is not supported; and two processes that were given different state folders for one checkout, which open two different databases and do not see each other. So one rule: everything that works on one checkout, one personal section, one host settings file, one index or one model cache uses the same state folder. That is the default; a person who sets `PBRAIN_STATE` sets it the same everywhere, a supervisor resolves it once for all its workers, and it is changed only with everything stopped, its contents moved with it.

### 12.6 Databases on disk

Memory and the semantic indexes are ordinary SQLite databases, opened in place, in WAL mode, with every commit synced to disk: a store writes one record, and any number of agents read while one writes. This holds on Linux and on macOS. The semantic index is no longer loaded into memory and written out whole; `sync` builds it in place, in transactions, and a reader sees one committed state.

What a search and `sync --status` see is what the pbrain spec (9.4, 7.1) promises today, and no more is deferred than today:

- **Every commit is a usable checkpoint.** After the first committed part of a first build, a semantic search uses the items indexed so far, with today's rules for missing and changed items and today's `semantic_partial` notice; `sync --status` counts them.
- **An interrupted `sync` resumes** from its last commit; what was committed stays in use meanwhile.
- **An index records what it was built with, in its own database**, not in a file beside it: the model's name; the size of its vectors; the vector profile, which is the fingerprint of the model's files and of the way pbrain cuts and prepares text, in the variant that made the vectors (ordinary, fp16 on NVIDIA, or Metal); the precision that was asked for; the form of the index; and whose it is (the project, the checkout or personal section, items or documents).
- **A search refuses an index that does not match.** Before any vector is compared, and without asking any device, a search checks that record against what the project's configuration asks for now. The check is today's and accepts what today's accepts (pbrain spec 9.4, 10): the same model and vector size, the same form, and a profile that the configuration allows, which includes either Metal variant and an index built on the processor or with ordinary precision because the GPU could not do what was asked. A changed precision setting is a mismatch only where that check rejects the index. The check is made for each index a search uses, separately: the shared items, the personal items, the documents. From an index that fails it no vector is compared with the query; only that index is left out. The indexes that pass still answer and are ranked together as today, also when they hold different accepted variants of one model, such as a shared index built with fp16 on a GPU and a personal one built on the processor. When at least one index answers and another was refused, the notice is `semantic_partial`; when none answers, the search returns its lexical results with `semantic_not_ready`. Either notice says what differs and that `pbrain sync` (or `pbrain docs sync`) is the fix. `sync --status` reports a refused index as built for another model or as unreadable, as today.
- **Another model or profile is built beside the index in use.** The index in use goes on answering only for a configuration it still matches, by the rule above; after another model is named it answers nothing: searches say `semantic_not_ready` until `sync` has committed the first part of the new index, and then use that as far as it is built, with `semantic_partial`. A half-built index for another profile never replaces a usable one that still matches.
- **One index, reused.** A checkout has one index database per kind, and every `sync` updates it in place: there is no file per build and no file per model. A build for another model or profile is prepared inside the same database, and when it is complete it takes over in one step, in which the superseded index is deleted as well. A search that began before that step finishes on what it was reading; one that begins after it uses the new index. Nothing is left for a person to clear, and the file may keep its size, since the freed room is reused.
- **A damaged index is rebuilt by `sync`**, with no person involved: when no search is reading it, `sync` removes the damaged database and builds it again from the vectors in the cache.
- pbrain never deletes the data of another checkout.
- A build that is killed leaves a database that opens and answers as far as it had committed.

**Reading needs the folder too.** SQLite keeps two side files beside a database in this mode and may have to create them even to read. So `recall` needs to write in the state folder, and a semantic search in the cache. On a folder that is read-only, `recall` fails with `io`, naming the folder, and so does `sync`. A search with `--semantic` keeps the pbrain spec's behaviour for an index it cannot use: it returns its lexical results with `semantic_not_ready`, and the notice names the folder. Only local file systems are supported.

SQLite opens its files by name, so these databases are the one place where pbrain does not reach a file through an open folder handle. That is confined to pbrain's own two per-user folders: the paths are built by pbrain alone; the folders and files are owner-only and are checked for type, owner, mode and links before every open; no row of a database is read into a result, and nothing is written to it by pbrain, until pbrain has found its own schema and the expected project, checkout or section in it (SQLite itself may read the file's header and complete an interrupted commit before that); and a restricted worker under a supervisor has access to neither folder, only its supervisor's broker has. Another user cannot interfere. A process of the same user that is allowed to write these folders is trusted with them: it could redirect an open, and pbrain does not claim to stop that.

### 12.7 Guards

- The **protected paths** of the pbrain spec (5) become: the personal section, the default `<brain>/personal/` even when another folder is configured, and `personal.toml`. A section's `.pbrain/` is no longer one of them, because there is none; an old one that is still present is pruned from every walk and refused as input until it is moved.
- The exposure guard on the personal section is unchanged.
- Much of the concern about links goes: pbrain no longer owns files inside a repository that someone else could have prepared. What stays: items and documents are still read through open folder handles, never through a link; inputs are still resolved and refused inside protected paths and inside the two per-user folders.

### 12.8 Who may write where

| Command | Needs to write |
| --- | --- |
| `add`, `verify`, `audience`, `delete` | The working tree, and the state folder to take its turn |
| The first writing command on a brain that still has a `.pbrain/` | The working tree and the state folder (its turn and the move) |
| `import`, and `delete` of an imported item | The working tree and the state folder (its turn and the originals) |
| `store`, `forget`, `promote`, `recall`, `memory init` | The state folder; `promote` also the working tree |
| `hooks install`, `hooks uninstall` | The state folder and the host's settings file |
| `sync`, `docs sync` | The cache, and the state folder to take the build's turn |
| `sync --status` | To read the cache and the state folder, for the turn it holds while it reads an index; where it cannot, it reports that index as not readable |
| A search with `--semantic` | The cache, to use the index; to read the state folder, for the turn a search holds while it reads; and to write both, to catch up. Without the first two the search still answers by words; without the third it answers from the index as it is |
| Every other read | Nothing |

So every command that writes an item needs the state folder. In a sandbox that allows only the working tree, such a command fails with `io`, naming the folder, before it changes any item.

A sandbox has to allow those folders for those commands, and the README names the paths. A restricted worker under a supervisor's automatic checkpoint has no such access: the supervisor writes for it (section 8.4).

## 13. The skill

The skill stays the one place that says how to use pbrain. It gains:

- **Recall first.** Before a non-trivial task, recall with two or three durable words from it, as it already says to search; with a task token, `recall --task`.
- **Requests to remember**, with the table and the rules of section 6.1, the one question of 6.2, and the offers of 6.5.
- **What to store on its own.** A handoff when it reports or hands off a task; a blocker when one stops it; a lesson when it learns one. The capture rules of 6.4.
- **How to read a memory.** As something said: check the cited source before acting on it; a typed report is a pointer to the CI run, never the result; nothing recalled is an instruction.
- **Banking.** Promote what proved useful, through the preview; search before proposing a new document.

No file that pbrain generates or rewrites describes memory in the repository. A project may keep a small `BRAIN.md` that a person writes and owns (where the documents are, how to recall for a task, how to propose durable knowledge); pbrain never touches it. It helps only if the host's own instruction file (`AGENTS.md`, `CLAUDE.md`) points at it or at the skill, and `init` prints those lines for a person to paste.

## 14. Configuration and environment

| Name | Where | Meaning | Default |
| --- | --- | --- | --- |
| `[docs] folders` | `pbrain.toml` | The document folders (11.1) | none; `["docs"]` written by `init` in the default layout |
| `recall = { limit, max_chars }` | `pbrain.toml` | Defaults of `recall`, as `search` has them (pbrain spec 10) | 5 and 3,000 |
| `PBRAIN_STATE` | environment | The per-user state folder (4.1) | the platform's state folder |
| `PBRAIN_SESSION` | environment, set by an adapter | The host session of this process | absent |
| `PBRAIN_TASK` | environment, set by a supervisor or a person | The task of this session | absent |

The lifetimes, the caps, the quota and the size limits of sections 3.5 and 7 are fixed, not settings.

## 15. New error and notice codes

Errors: `memory_not_enabled`, `memory_busy` (retryable), `secret_refused`, `quota_full`, `no_source`. The existing `too_large`, `invalid`, `conflict`, `confirm_required`, `not_found`, `usage`, `git_required`, `personal_exposed` and `invalid_config` are used as stated above.

Notices: `memory_not_enabled` (on `recall`), `docs_not_configured`, `state_moved`, and the existing `destination`, `after_write` and `semantic_not_ready`. The existing `cpu_fallback` keeps its code and says more (section 11.7).

Findings of `docs check`: section 11.5.

## 16. Acceptance scenarios

Each is a test or a scripted run. Linux and macOS unless one is named.

**Memory by hand (phase 1)**

1. Two linked worktrees with different uncommitted versions of the same managed item: each reads its own text, and both recall the same one task memory.
2. An external brain, and a second clone: each has its own memory; neither sees the other's.
3. Concurrent stores from five agents: every acknowledged memory is there exactly once; a reader during a write sees a complete state.
4. Replay: the same `--event` and content returns the first id; changed content is `conflict`; a replay after forget writes nothing.
5. Text is stored byte for byte: a multi-paragraph answer of 20,000 characters with Markdown, trailing spaces and CRLF comes back identical with `recall --id`, in text and from the JSON pages joined in order. `--id` with `--page`, `--page` with `--limit`, and `--id` with `--banked` are usage errors.
6. Refusals: 65,537 bytes is `too_large`; a planted token is `secret_refused` and neither the message nor any file holds it; invalid UTF-8 is `invalid`. Nothing is written in any of them.
7. A short-term memory, a blocker included, is gone after seven days; a long-term one is there after fifteen days, a restart, quota pressure and a cache purge, and gone after `forget`. A long-term one with `--until` survives the same pressure and is gone at its date, not before.
8. The 33rd short-term memory of a task drops the oldest; a long-term memory of the same task stays; a replay of the dropped one is refused. Thirty-three no-task stores across two worktrees under different claimed authors leave 32, and leave the personal section untouched.
9. The quota's two allocations: with the ordinary allocation full by count, and separately full by bytes, an agent's long-term store is `quota_full`, a short-term store still works, and a store at a person's observed request is still admitted; a store that only claims `--asked-by human:owner` is counted as ordinary; the 201st observed person's request is `quota_full`; a duplicate is not counted twice; after one `forget` a store works again. Run with concurrent writers.
10. Private data: with and without planted personal memories, every shared result, count, message and file is identical.
11. Without `memory init`: `store` fails with `memory_not_enabled`; `recall` returns an empty page with the notice and exit 0.
12. `--learned --retention short-term` and `--learned --kind outcome` are usage errors; `--learned` with no other flag makes a long-term statement marked learned.
13. Typed reports: `--fact check-passed --source factory` from the CLI and from a tool call, and the same from a supervisor, are shown identically as "Unverified: factory reports …", rank after an authored handoff, and keep their label when the page is cut.
14. Provenance: `recall --id` shows each field with `observed` or `claimed`. No flag sets the model, effort, harness or source commit: with the adapter's session noted the model is `observed`; with no adapter all four are absent, not guessed. `--said-by` and `--asked-by` given on the command line are `claimed`.
15. Labels: a memory marked said, a provisional note, an imported source, a verified decision and a typed report appear side by side with `recall --banked`, each with its own class and status, and no existing command's output differs from before.

**Explicit capture (phase 1)**

16. A scripted agent with the skill, given each line of the table in 6.1, makes the stated kind and term; given a quoted instruction, a negation, a question and "remember when …", it stores nothing; given "remember this exact quote: …", it stores the quote.
17. "Remember that X" inside a longer prompt stores X and nothing else.
18. `--from-last-response` on the qualified host stores the previous visible answer byte for byte, with `said_by` the agent, `capture: verbatim` and its `turn`. Two model cases: the session starts on model A, switches to B, and a B answer is recorded as B or left absent, never as A; and an older A answer recorded after the switch is A or absent, never B. After a branch change the `recorded_` fields name the new checkout and are labelled as the recording checkout, and `source_commit` is the old commit if the adapter observed it and absent if not. After the session's task changed it fails with `no_source`. `--from-last-response --session OTHER` is a usage error, and with `PBRAIN_SESSION` forged to another live session's id it fails with `no_source` and reads nothing of that session. `--from-last-response --capture checkpoint`, and `--capture checkpoint --learned`, are usage errors.
19. With no adapter, `--from-last-response` fails with `no_source` and stores nothing; it never returns an older answer.
20. The dialog: answered *for the team* stores shared with `asked_by` the person; *only for me* stores personal. `store --dry-run` before the dialog prints its selection line with no memory opened or created in either section, and only the store after a *for me* answer is counted, in the personal section. On a host with no dialog, "remember only for me that X" and "we learned X" from an unlabelled message, each with a private sentinel in X, leave no row, receipt or text in either section and no trace of the sentinel in any file or message; the reply carries the command with the settled kind and term, and running it stores the memory. An unlabelled "yes" and a reply from automation that claims to be a dialog answer store nothing.
21. Lessons between agents: one message with one lesson delivered to two workers gives one long-term memory, said by the teacher, asked and recorded by the designated receiver; delivered twice under the same lesson id, still one; a message with two lessons gives two; an independent message with identical words gives another; and a replay after the memory was forgotten writes nothing. An agent paused by a planted private item that learns X from it stores nothing in the shared section; an agent that learns the same words independently in a clean shared context stores them there.
22. Enabling explicit capture creates no additional copy of any answer in pbrain's or the adapter's files or logs.

**Promotion (phase 2)**

23. Preview and confirm an authored memory as a provisional learning, and another as a decision with a decider: the published bytes equal the preview.
24. A typed report: preview as a decision with `--decided`, as a learning with edited text, and a confirm with a well-formed token are each `usage`, with no token and nothing changed.
25. The five rows of the failure table in 10.1, each as its own test, single and grouped; in particular a refused sync after the spent mark is visible publishes nothing, on the first call and on a retry.
26. Three memories into one learning: each source appears in the Sources part with its ref or its quoted excerpt; a second clone with no memory reads every citation. Forget one source before confirming: the whole proposal is refused. A source whose ref is a memory id or a path outside the project, with no quote: `invalid`; the same source quoted: accepted. A personal source with a shared target: refused; a wholly personal proposal: allowed.
27. After promoting an edited draft, `recall --id` returns the exact original. Preview a group that quotes memory A, forget A from another worktree, then ask `--show` for page 2 with the still-unexpired token: no bytes, `not_found`. The same after A expired, and for a personal token presented without `--personal`.

**Long sessions (phase 3)**

28. The task page appears after a start, a resume, a fork and a compaction on the qualified host, once each, with at most three rows and 1,200 characters; a memory forgotten just before a re-injection does not appear. A fork, and an unknown session that inherits `PBRAIN_TASK`, with personal memories planted for that task, receive no personal row.
29. With no task, or memory not enabled, or pbrain missing, the hooks return nothing and the session works.
30. (With the adapter, in phase 1.) `hooks install` on a project with existing settings shows its change and keeps every other setting; `uninstall` restores an untouched file byte for byte, keeps an unrelated later edit, and reports a `conflict` for a pbrain entry the person edited.
31. Checkpoints, with a supervisor: one handoff yields one prompt; idle, retries and unrelated ends of turn yield none; storing nothing is accepted.
32. The pause, five cases: a labelled assignment leaves its receiver eligible; unmarked confidential input to the operator withholds only the operator's checkpoint and leaves its explicit saves working; the same pane after a compaction is still paused; a clean worker that got only the assignment still checkpoints; a delivery that mixes labelled and unlabelled text pauses its receiver. A paused worker cannot store by writing the state folder itself.
33. Hook cost: section 17.2.

**Documents (phase 5)**

34. Compatibility: two matched item fixtures with a fixed clock and ids, one with an enrolled folder holding a valid document and an invalid-UTF-8 file. `search`, `list`, `get`, `prime`, `check` and `sync`, from the CLI and through the catalog, give the same output, findings, exit code and counts on both. Only `docs check` reports the bad file.
35. The default layout: `docs/` enrolled, `docs/brain` managed, a private sentinel in `docs/brain/personal`. No `docs` command reads, counts or names the sentinel; every item is returned once, as an item. `docs inventory docs`, from the CLI and as a tool, gives the same output, counts, notices and paths with and without the sentinel.
36. A file dropped into a folder is found by `docs search` at once; renamed, it is found at its new path and not the old; deleted, it is gone.
37. Headers: no header, a foreign header with a title, a broken header, and a header with `status: verified` and `decided:`. Each is a document; none has a status; the last never appears in `prime`.
38. Two `README.md` files in different folders are two results; two paths that differ only by case are left out and reported. A managed item whose id is `report.md` is read by `docs get report.md`, and every path and id that `docs search` and `docs list` return opens with `docs get`. With no `[docs]` table, `docs sync` and `docs check` succeed with `docs_not_configured`, and `docs search --class document --semantic` needs no item index.
39. Two worktrees at different commits each search their own documents, and `docs sync --status` is separate for each. A `docs sync` leaves `sync --status` unchanged. After the move of 12.4, four cases for the first `sync`: an intact cache (it embeds nothing for shared passages whose vectors it can read), a cleared cache (it embeds everything), another profile selected (it embeds only what the cache lacks for that profile), and a personal section (it embeds what its own vectors lack and takes none from the shared ones). `--personal` reports each side's readiness by itself.
40. Order: a document whose title holds every query word comes before an item that holds them only in its body; inside one match class the items come before the documents.
41. The quality gate of section 17.4, run from the frozen fixture: its three hashes are checked first, and a mismatch fails the gate before any search runs.
42. Promotion into a new document and a patch for an existing one, as in 10.4; machine C with an empty memory finds the banked document after a pull. A patch whose one source has no portable ref, with the same complete body given once on the command line and once as a tool call with `text` and `quote`: both patches are identical, each includes the full quoted text with its attribution, and the runbook on disk is byte for byte unchanged.

**Out of the working tree (phase 4)**

43. A fresh `init`, then `add`, `import`, `sync`, `store` and the same with `--personal`: there is no `.pbrain/` in the brain or in the personal section, and `git status` shows the items and nothing else of pbrain's.
44. Moving an existing brain: with originals, an index and a lock file in `.pbrain/`, the first writing command copies every original, verifies it, prints `state_moved`, and the old folder is gone; a read before the move, and one racing it, still finds its original; two first writers at once both succeed with one cutover, and neither creates an old lock file or an old folder again; a failed sync of a copied file, of a new folder or of the receipt leaves every old original and fails the triggering command; a process killed after the copy and before the receipt loses nothing, and the next writing command finishes the move; one killed after the receipt and during the clean-up reads from the state folder, finishes the clean-up, and does not bring back an original deleted meanwhile; an original that does not match its digest stops the move with nothing removed; a file that is not pbrain's leaves the old folder in place and the move is not tried again.
45. Worktrees and clones: delete an imported item in worktree A, and worktree B's item with the same digest still reads its original. Two clones of one repository at one commit, each with its own personal section, share no memory, no originals and no index; pointed at one personal folder, they share that section's data and nothing else. A worktree deleted and created again at the same path starts with no originals and no index of its predecessor's.
46. Purge and placement: after the cache is deleted, originals, memories and receipts are intact and `sync` rebuilds the index; the README states what deleting the state folder loses. With `PBRAIN_STATE` and `PBRAIN_CACHE` equal, one inside the other, the same through a link, or either inside a working tree: `store`, `import`, `sync` and the move each fail with `invalid_config` before creating, moving or deleting anything, and `search`, `get` and `list` work. After `PBRAIN_STATE` is changed to another folder, a shared `import` and a shared `--body-file` aimed at a personal original in the previous folder, directly and through a link to it, answer exactly as for a file that does not exist, return no byte of it and publish nothing; the same holds for the record that `hooks install` keeps of a settings file; an ordinary file outside these folders is still accepted.
47. Personal: with the personal section's state unreadable, every shared command works and reveals nothing of it; a personal vector is never served to a shared search.
48. Database files, for memory and for each index: a link at the database's name or at either side file's name is refused with `io` and nothing is opened; a real pbrain database of another project, checkout or section put in its place is refused before any of its rows is returned or anything is written to it by pbrain; with the folder read-only and the side files absent, `recall` and `sync` fail with `io` naming the folder, and a search with `--semantic` returns its lexical results with `semantic_not_ready`; five writers and readers at once, on Linux and macOS; a process killed before and after a commit leaves a database that passes SQLite's integrity check; for the index, on each of three branches (a first build, an incremental `sync` for the same profile, and a build for another profile): what a search and `sync --status` report after the first commit, after the writer is killed, and after a successful resume, with a reader running throughout; and a half-built index for another profile never answers in place of a usable one. After a build for another profile completes, the superseded index is gone from the same database in the same step, there is still one index file for that checkout and kind after repeated changes of profile, a search that began earlier finishes on what it was reading, and another checkout's index is byte for byte untouched; a damaged index is removed and rebuilt by an ordinary `sync` once no search is reading it.
49. Sandbox (and, for the rule of 12.5, two processes that resolve one state folder, also through a link to it, taking turns on one checkout): with only the working tree writable, `add`, `verify`, `import`, `store` and `recall` each fail with `io` naming the state folder, and no item is created, changed or removed; `search`, `get` and `list` work. With the state folder writable as well and the cache not, `add` and `verify` succeed. A shared write succeeds when nothing of the personal section can be opened. With no setting at all, every command works on Linux and on macOS.
50. Indexing on the processor, with a test model that has a GPU path, a list of devices made for the test and a processor rate set for the test. With the GPUs made unusable, `sync` exits 0 and prints `cpu_fallback` once for the shared index, before its first passage, as a warning with the reason, a count equal to the number of passages the test model is then asked to embed, on a fixture that also holds a damaged cached vector (counted), a vector that exists only under another item that this run replaces (counted as the build treats it) and a passage that occurs three times (counted once), and the estimate that count and rate give; with the shared cache filled by another checkout after the warning, the model is asked for fewer; with `--json` the notice carries `section`, `reason`, `passages`, `passages_per_second` and `estimated_seconds`, it is valid under the catalog's description of a notice, and the same call made through the tool catalog returns the same notice. Each reason is shown in its case: `--device cpu` and `PBRAIN_DEVICE=cpu`, each named as the source; a build with no GPU support; no usable GPU; a GPU that cannot be started; too little free memory; and, under `auto`, a GPU that runs out during the run, in the middle and in the last group: the warning follows `embedding_device` when that group is done and counts only what is planned after it, which in the last group is nothing, said without an estimate. A run that fails or is cancelled after the warning ends with the error and exit code it would have had without it. A run that resolves its device and then finds nothing to embed prints no warning. A run with shared and personal items to embed prints one warning for each, with its own count; `docs sync` prints its own. Passages whose vectors are in the cache are not counted, and a run that embeds nothing prints no warning. A model with no measured rate gives the count and no estimate, and its JSON has no `estimated_seconds`. `search --semantic` and `docs search --semantic`, with and without a catch-up, never print `cpu_fallback`. And the record of 12.6: after a build, the index's own database names the model, the vector size, the profile, the asked precision and the form, and no file beside it does; with another model named in `pbrain.toml`, and separately with the precision setting changed to one that today's check rejects for that index, `search --semantic` returns its lexical results with `semantic_not_ready` and compares no vector, and `sync --status` says why; a precision change that today's check accepts changes nothing; with the item index matching and the document index built for another model, `docs search --semantic` still answers from the items by meaning, compares no vector of the document index and prints `semantic_partial`; a shared index built with fp16 on a GPU and a personal index built on the processor under the same setting both answer one search; none of these asks a device; after `sync` and `docs sync` every search answers by meaning again.

## 17. Constraints and delivery order

### 17.1 Order

Each phase can be released separately; nothing is tagged without the owner's word.

1. **Explicit memory.** `memory init`, `store`, `recall`, `forget`, on an on-disk database; the two terms, the caps and the quota; provenance; the skill's handling of requests to remember. Then the adapter for Claude Code, so that `--from-last-response` and observed provenance work: `pbrain hook session-start`, which in this phase only notes the session, and `hooks install`.
2. **Promotion** into a learning or a decision, one memory and then several.
3. **Long sessions.** The task page, returned by the session-start hook, and the `pre-compact` hook; then, in the factory's repository, the labelled channel, the checkpoint and the pause. Automatic checkpoints are on by default only after the pilot of 17.3 passes.
4. **Out of the working tree.** No `.pbrain/` in the repository: originals and locks to the state folder, the semantic index as an on-disk database in the per-checkout cache, and the move of existing brains (section 12).
5. **Documents.** `[docs]`, the `docs` commands, the document index, and a document as a promotion target.

Phases 3 and 4 do not depend on phases 1 and 2 or on each other. Phase 5 needs phase 4.

### 17.2 Hook cost

Compare 1,000 matched ordinary ends of turn, with small and 64 KiB answers, on Linux and macOS, with the adapter off and with it on and the checkpoint feature enabled but no handoff eligible, including the real adapter launch. Report the 50th, 95th and 99th percentiles against the budget of 9.4, and show zero durable writes, zero model calls and zero prompts on that path. A hook that overruns its limit is abandoned and the turn completes. The session-start budget is measured the same way at 10,000 memories.

### 17.3 The pilot before checkpoints are on by default

- **Fixed control:** the skill and today's managed items, no memory. At least six tasks and 24 paired handoff cases; the model, build, fixture, task order and budgets held constant.
- **Two arms**, each against that control and reported apart: explicit memory alone; then with the task page and checkpoints. Both arms get the same "remember" requests.
- **Cases** include changed and contradicted evidence, a question only a private memory could answer, duplicate reports, and a fresh agent meeting the same problem again after the short-term memory has expired, which must find the banked lesson by its Git citation.
- **Measures:** correct next action; repeated investigation; wrong durable decisions; false carry-over; unused or harmful memories; all tokens spent on prompts, checkpoints and re-injections; human review time; the share of sessions that were not eligible; and the number of memories a task collects in a week, which sets the cap of 7.1.
- **Gates:** no private or secret leak; no unauthorized promotion; no increase in wrong durable decisions; and, for the automatic arm, at least four more correct resumptions in 24 than the control. An inconclusive result is reported as such.
- Any paid run needs the owner's approval, with a figure.

### 17.4 The document quality gate

The gate is fixed now, before any document code is written. Its corpus, its questions and their expected answers are in `specs/projects/pbrain-2/fixtures/docs-quality/`, which is part of this spec:

| File | SHA-256 |
| --- | --- |
| `fixtures/docs-quality/README.md` (the manifest: every document, its origin and its hash) | `99f4421827db9776a4d292403047e1334284f1a6f34e00be2e2ed8db49c27f8d` |
| `fixtures/docs-quality/questions.json` (32 questions with expected paths, ranks, evidence lines and snippet locations) | `e4c53b0236a0c2bffe7a1f5551c9fddc8dfef1c4fe1b6b642900bf4ea39c0612` |
| `internal/semantic/calibrate/testdata/retrieval/questions.json` (the existing questions, unchanged) | `af4054bbb01300f9f3097b3b414f4a4a5f05d246fa62b2cd48ea906358b5c041` |

A change to any of them is a change to this spec and needs the same review.

**The corpus.** Thirteen documents, all present as files in the fixture's `tree/` folder, so the gate needs no Git history and runs from a shallow checkout. Six are byte-for-byte copies of committed research documents; four of these are the long documents, each over 12,000 characters, with every one of their questions answered in the last quarter of the file. Seven were written for the fixture: two files named `README.md` in different folders; one with a foreign header that has a title and no heading; one with a header that is never closed; one whose header imitates an item's standing; and two that share words with fixture items and answer none of their questions. With the two copied files named `research_summary.md` that makes two equal-name pairs. No document has pbrain front matter. The managed area beside them is the existing retrieval fixture's 35 items.

**The questions.** Fourteen keyword queries, each with an expected path and rank, the lines that hold the evidence, and the exact line and offset where the snippet must start; two keyword queries that must return nothing, because their word occurs only in a header key that is not searched; twelve questions in plain language for `--semantic`, each with an expected path and its evidence lines; and four questions that nothing answers. A location line says where a snippet starts, which is not where the answer is, so the fixture keeps the two apart. The keyword entries were computed with the snippet rule of pbrain spec 7.1 and their offsets checked against the product built from commit `d9a9c3e`; the semantic entries could not be checked, since no model was run.

**How it is run.** In a scratch Git project with the thirteen documents as `docs/`, the 35 items as `docs/brain` and `[docs] folders = ["docs"]`; with the calibration tool's fixed clock, the shared section, and the release's default model and budgets. The thirteen document hashes and the three hashes above are checked first, and a mismatch fails the gate before any search runs. Two modes of the public command are gated, and no other: `docs search` with no flag, and `docs search --semantic`. `docs search` has no `--bm25` in the first version (section 18), so that arm, and the arm that combines it with `--semantic`, are not part of this gate; the calibration tool's own comparisons on items are unchanged and stay where they are.

**It passes when all of these hold:**

1. **Nothing existing gets worse.** For every existing question, in each of the two modes, the legacy outcome is what `search` returns on the 35 items alone, on the same build, read to the end of its list.
   - *Items, whatever the page.* `docs search --class item` returns exactly the legacy list: the same ids in the same order.
   - *The visible page.* On the default page of `docs search`, with the documents enrolled: if the legacy first page holds an expected item, this page holds it too, and no document comes before the first expected item. Where the legacy outcome itself lacks the expected item, nothing is asked about that item; that miss is not this gate's.
   - *No new false hit.* For every existing question that nothing answers and on which the legacy outcome is empty, `docs search` returns no item and no document. The gate claims nothing about a question on which the legacy outcome was already not empty.
2. **Keyword.** Each of the fourteen keyword queries, asked with no flag, returns every expected path within its stated rank, with exactly the stated location line and body offset. The two `keyword-nothing` queries return nothing.
3. **Meaning.** With `--semantic`, each of the twelve questions returns its expected path among the first five results, and the passage whose start the result's location names contains the evidence lines: the location is at or before the first of them and that passage runs to at least the last. Each of the four new unanswerable questions returns nothing at all, with no allowance.
4. **Reported, not gated:** the passages and time to index the folder on the named machine, and the time of a plain `docs search` over it.

A failure of 1 or 2 is a defect. A failure of 3 on a single question is brought to the owner with the scores, because it may be the model's cut-off and not the code; the fixture is not edited to make it pass.

### 17.5 Other constraints

- Memory needs no model, no network and no index. Warm lexical recall is within 100 ms at the 95th percentile with 10,000 memories.
- Supported sharing is one operating-system user on one machine with a local file system.
- Linux and macOS on Apple Silicon, as for pbrain today.

## 18. Out of scope

- Harvesting transcripts, tool output or direct messages; a memory written on every turn; a copy of every answer kept in case someone asks; a background process that writes memories, documents or skills.
- Copying unreviewed memories between machines or accounts. A live handoff between machines before review is a separate requirement if the owner names it.
- Semantic recall of memories, an importance score, ranking by how often something was read, and an LLM that summarizes memories.
- A pattern or hook that decides from a prompt's text that something should be remembered.
- An in-memory helper that holds the latest answer for hosts with no record to read, and any staging file on disk for it.
- A setting that treats whatever is typed into a pane as team-visible.
- Treating a plain document or an unpromoted memory as a managed item; results that mix classes without a label.
- Personal document folders; documents for a brain outside Git; `--bm25` on `docs search`.
- A file generated by pbrain that describes memory, in the repository.
- Hosts other than Claude Code, until each is qualified.
- Automatic creation or editing of skills and host instructions.

## 19. The basis of this spec

**The owner's decisions.** D1 to D20 of `proposal.md`, all as recommended, with later ones that replace D5's and D10's detail: nothing of pbrain's in the working tree; on-disk SQLite for memory and the indexes; and no lock files, with SQLite transactions for taking turns (section 12). Answers of 2026-11-26: a standing rule is offered for banking (6.5); one dialog question settles who and where, and the wording sets the term, with "learned" always long-term, for agents too (6.1, 6.2, 6.4), except that the round 1 fallback for a host with no dialog is replaced, which the owner confirmed (section 20); the long-term quota is 500 memories and 16 MiB (7.2); `statement` is the kind of a remembered statement (3.2); `pbrain.toml` stays in the brain folder (11.1); `docs search` orders by match class with items ahead of documents inside one class (11.5).

**Settled in this spec, and accepted by the owner in round 1:** the flag `--retention` with `--until`; `--said-by`, `--asked-by` and `--author`; `recall`'s `--banked`, `--id`, `--page` and `--retention`; a task as a token; `memory_not_enabled`; `PBRAIN_STATE`; a document's address as its path; `docs inventory` and no rewriting of `pbrain.toml`; shared-only document folders; `hooks install` writing the untracked settings file by default; the document quality gate, which round 1 left as a shape and which is now a frozen fixture with pinned hashes, at the reviewer's request (17.4).

**Settled in this spec, new since round 1, for the reviewers to check:**

- Something learned as a mark on a `statement`, not a kind of its own (3.2), which the owner asked to have considered and both reviewing designers recommend.
- The phrase table of 6.1, and that the agent matches it under the skill's rules, not a hook.
- That a lesson is stored by exactly one writer, identified by the teaching event's own id and never by searching for its words (6.4).
- `--ref`, and the rule that a promoted source needs a portable ref or a quoted excerpt (10.3).
- `secret_refused`, `quota_full`, `no_source` and `memory_busy` as new codes.
- That pbrain ships two hooks, `session-start` and `pre-compact`, and that the checkpoint's end-of-turn hook belongs to the supervisor (8.4, 9.1). D20 said every host calls `pbrain hook`; that holds for the events pbrain owns.
- That a change to an existing document is returned as a review-only patch and is not recorded as a promotion (10.4); `promote --show`; `docs get --path`; `docs_not_configured`; `--capture checkpoint`; the `recorded_` and `source_commit` fields; and that no personal row is injected in the first version (8.1).
- The ceilings: 16 folders, 16 sources in one promotion, 10,000 short-term memories per section.

## 20. Questions the owner answered, and what is left for the architecture

The owner answered these on 2026-11-26, when approving this spec:

1. **Agents' lessons and the long-term quota.** Split the quota: a reserve for memories a person asked for, and the rest for agents. Section 7.2 has it. The reserve is for an observed human requester; the factory's operator agent is an agent.
2. **Learned as a mark, or as a kind of its own.** The mark on a `statement`, as in section 3.2. No `lesson` kind.
3. **A host with no dialog.** Nothing is stored, and the agent shows the exact command, as in section 6.2. This replaces the fallback of round 1.
4. **The dialog on every "remember" typed by a person.** Accepted: one click. To be looked at again only if it irritates in use.
5. **Standalone use has no automatic checkpoint.** Accepted for the first version.

6. **Later decisions of 2026-11-26.** No `.pbrain/` folder in the repository, for the shared brain or the personal section; rebuildable data in the per-user cache, memories and originals in the per-user state folder; and ordinary on-disk SQLite, opened in place, for memory and for the semantic index, on Linux and macOS (section 12).

The owner then confirmed both, and said that what makes writers take turns belongs in the per-user state folder, with the originals and the memories (12.5). So every command that writes an item needs the state folder (12.5, 12.8). The owner then decided, on 2026-11-27, that pbrain keeps no lock files at all: turns are taken through SQLite, on a small database per section in the state folder (12.5). There is no separate lock folder and no look-up of the user's account, and everything that works on one checkout uses the same state folder.

The owner's last two answers:

- **A typed "remember", before pbrain can tell a person's answer to the dialog from an automated one:** an answered dialog is enough to store. The memory is recorded with `asked_by` claimed and counted in the ordinary allocation until a host version is qualified (6.2).
- **Data of a worktree that no longer exists:** its index and its originals stay behind. The layout is documented, pbrain never deletes another checkout's data, and a command that lists and removes such data is a later addition.

**Asked by the owner on 2026-11-27, and added:** that indexing on the processor is announced as a clear warning with the reason and an estimated duration (11.7), while a search on the processor stays silent; and, stated outright, that each index records its model, profile and settings in its own database and that a search refuses an index that does not match (12.6).

For the team, to settle in the architecture step: the schema of the on-disk indexes and how a build stays complete or unused; how the adapter reads the previous answer and the effort from each host; how a supervisor gives a long-lived agent a clean context for its next task; and the byte limits on a section's stored memory and on its replay receipts, which must be exact.
