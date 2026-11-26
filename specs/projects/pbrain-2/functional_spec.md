---
status: complete
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
- The tool names and meanings in the tool catalog (`pbrain schema`) do not change. New commands are new tools. JSON only grows (pbrain spec 6.2).
- A brain with no memory enabled and no `[docs]` key behaves exactly as today in every respect, with one planned exception stated in section 12.3 (the item index moves, and needs one `pbrain sync`).
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

Memories are originals: nothing can rebuild them. So they are not kept in the cache.

- **Shared memory** lives in the per-user state folder: `PBRAIN_STATE` if set, else `pbrain` under `$XDG_STATE_HOME` when that is an absolute path, else `~/.local/state/pbrain` on Linux and `~/Library/Application Support/pbrain` on macOS. Inside it: `.pbrain/memory/v1/<project-key>/`. The `.pbrain` component is deliberate: `import` and `--body-file` already refuse any input whose resolved path contains it, so a memory can never be read back in as content.
- **Personal memory** lives under the resolved personal section, never in the shared state folder. A shared call never opens, counts, hashes or locks it, and nothing about it can change a shared result or error.
- **The project key** joins all linked worktrees of one repository for one brain: it is derived from the repository's Git common directory and the brain's path inside the work tree. A second brain in the same repository, a fresh clone, another user and another machine each have their own memory. Branch names and remote URLs play no part. A brain outside Git is keyed by its own resolved path.
- Clearing the cache loses no memory. Deleting the state folder loses all of it, and pbrain says so in `memory init` and in the README.
- pbrain writes nothing into Git's own metadata.

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
             [--session ID] [--event UUID --observed-at TIME] [--personal]
```

- **The text** comes from exactly one place. `--text` takes it as an argument. `--stdin` reads standard input to its end and keeps every byte, which also keeps private text out of the process list. `--from-last-response` asks the host's adapter for the previous complete, visible answer of the session (section 6.3). `--fact` makes a typed report (3.3).
- **Which entry point takes what.** The command line takes all four. A tool call takes `text` or `fact` only: a tool has no standard input and no host session of its own, so `--stdin` and `--from-last-response` are not in the tool's schema. An adapter or a supervisor uses the command line.
- **Combinations that are usage errors.** More than one text source, or none. `--fact` with `--kind`, `--learned`, `--retention`, `--until`, `--capture`, `--said-by`, `--asked-by` or `--ref`: a report has fixed wording, is short-term and is its own capture. `--until` with short-term. `--learned` with short-term or with a kind other than `statement`. `--from-last-response` with `--capture` or with `--session`: it is always verbatim, and its session is the adapter's, never the caller's. `--capture checkpoint` with a kind other than `handoff`, `outcome` or `blocker`, or with `--learned`, `--retention long-term` or `--until`: a checkpoint is a short-term note. These are refused before standard input or any source text is read.
- **A checkpoint** is stored by a supervisor with `--capture checkpoint`, which requires `--task`, `--source`, `--session`, `--event` and `--observed-at` and applies the 2,048-byte limit. Any caller may pass it; it only lowers the limit and labels the row, and grants nothing.
- **Kind and term.** `--kind` defaults to `outcome`, and to `statement` with `--from-last-response` or `--learned`. `--retention` defaults to the kind's term (3.2). `--until` gives a long-term memory a chosen end, in the forms of pbrain spec 7.1 for a date or an instant; it is a usage error with short-term.
- **Who.** `--said-by` is who said the text, default the recorder. `--asked-by` is who asked for it to be kept; leave it out when the recorder decided by itself. `--author` is the recorder, resolved as `add` resolves it; with no author it fails with `invalid`.
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

- **A request that arrived as a prompt.** The agent asks one question through the host's own question dialog: *for the team* or *only for me*. The answer settles who is asking and where the memory goes; it does not ask about the term. *For the team* stores in the shared section, *only for me* with `--personal`. The memory records `asked_by` as that person. If the request already names the destination ("remember for the team that …"), the dialog still confirms it with one click, because a request to remember is not by itself permission to share.
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

A long-term memory stays until someone forgets it or until the date its requester chose with `--until`. It is never dropped to make room, never removed by promotion, and untouched by a cache purge. Each section holds at most 500 long-term memories and 16 MiB of their text, whichever is reached first. When either is full, a new long-term store is refused with `quota_full`, the message says how to list them (`pbrain recall --retention long-term`), and `forget` still works. A short-term store is never refused for this reason.

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

Available with the documents phase (section 17.1).

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

## 12. Where state lives

### 12.1 The table

| State | Where | Shared between worktrees? |
| --- | --- | --- |
| Documents and managed items | The checkout, in Git | No: each checkout has its own files |
| Originals of imports; write locks for items and settings | Beside what they belong to, as today | No |
| The item index and the document index | The per-user cache, keyed by project and checkout | Never: two worktrees hold different bytes at one path, even at one commit |
| Model files and the vectors of shared passages | The per-user cache, by content, as today | Yes: identical text embeds once |
| Personal derived data and personal memory | With the personal section | No |
| Shared memory | The per-user state folder (section 4) | Yes: every worktree of one repository, for one brain |

### 12.2 Who may write the state folder

A person's own session and an agent run by hand may write their brain's state folder and the cache; a sandbox has to allow both, and the README says which paths. A restricted worker under a supervisor's automatic checkpoint has no such access: the supervisor writes for it (section 8.4).

### 12.3 The one upgrade step

The shared item index moves from the brain's `.pbrain` folder to the per-user cache with the documents phase. The personal index stays where it is, with the personal section, and stays ready if it was ready for the profile in effect. After that upgrade, in each checkout, `search --semantic` returns its lexical results with `semantic_not_ready` for the shared section until `pbrain sync` has run once there. `--personal` is still a combined view: it reports the shared side as not ready by itself, and the personal side as it is. That first shared sync reuses every vector the cache holds for the same text and profile and embeds only what is missing: nothing, when the cache is intact and the profile unchanged; everything, when the cache was cleared; and after a change of profile, whatever the cache does not already hold for that profile from an earlier run. The old index files are left in place and can be deleted by hand. Nothing else about an existing brain changes.

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

Notices: `memory_not_enabled` (on `recall`), `docs_not_configured`, and the existing `destination`, `after_write` and `semantic_not_ready`.

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
9. The 501st long-term store is `quota_full` and a short-term store still works; after one `forget` a long-term store works again.
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
20. The dialog: answered *for the team* stores shared with `asked_by` the person; *only for me* stores personal. On a host with no dialog, "remember only for me that X" and "we learned X" from an unlabelled message, each with a private sentinel in X, leave no row, receipt or text in either section and no trace of the sentinel in any file or message; the reply carries the command with the settled kind and term, and running it stores the memory. An unlabelled "yes" and a reply from automation that claims to be a dialog answer store nothing.
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

**Documents (phase 4)**

34. Compatibility: two matched item fixtures with a fixed clock and ids, one with an enrolled folder holding a valid document and an invalid-UTF-8 file. `search`, `list`, `get`, `prime`, `check` and `sync`, from the CLI and through the catalog, give the same output, findings, exit code and counts on both. Only `docs check` reports the bad file.
35. The default layout: `docs/` enrolled, `docs/brain` managed, a private sentinel in `docs/brain/personal`. No `docs` command reads, counts or names the sentinel; every item is returned once, as an item. `docs inventory docs`, from the CLI and as a tool, gives the same output, counts, notices and paths with and without the sentinel.
36. A file dropped into a folder is found by `docs search` at once; renamed, it is found at its new path and not the old; deleted, it is gone.
37. Headers: no header, a foreign header with a title, a broken header, and a header with `status: verified` and `decided:`. Each is a document; none has a status; the last never appears in `prime`.
38. Two `README.md` files in different folders are two results; two paths that differ only by case are left out and reported. A managed item whose id is `report.md` is read by `docs get report.md`, and every path and id that `docs search` and `docs list` return opens with `docs get`. With no `[docs]` table, `docs sync` and `docs check` succeed with `docs_not_configured`, and `docs search --class document --semantic` needs no item index.
39. Two worktrees at different commits each search their own documents, and `docs sync --status` is separate for each. A `docs sync` leaves `sync --status` unchanged. The upgrade of 12.3, four cases: an intact cache (the first sync embeds nothing), a cleared cache (it embeds everything), a personal section (ready throughout if it was ready, its index not moved, while `--personal` reports the shared side not ready by itself), and another profile selected (it embeds only what the cache lacks for that profile); the old files are still there after each.
40. Order: a document whose title holds every query word comes before an item that holds them only in its body; inside one match class the items come before the documents.
41. The quality gate of section 17.4, run from the frozen fixture: its three hashes are checked first, and a mismatch fails the gate before any search runs.
42. Promotion into a new document and a patch for an existing one, as in 10.4; machine C with an empty memory finds the banked document after a pull. A patch whose one source has no portable ref, with the same complete body given once on the command line and once as a tool call with `text` and `quote`: both patches are identical, each includes the full quoted text with its attribution, and the runbook on disk is byte for byte unchanged.

## 17. Constraints and delivery order

### 17.1 Order

Each phase can be released separately; nothing is tagged without the owner's word.

1. **Explicit memory.** `memory init`, `store`, `recall`, `forget`; the two terms, the caps and the quota; provenance; the skill's handling of requests to remember. Then the adapter for Claude Code, so that `--from-last-response` and observed provenance work: `pbrain hook session-start`, which in this phase only notes the session, and `hooks install`.
2. **Promotion** into a learning or a decision, one memory and then several.
3. **Long sessions.** The task page, returned by the session-start hook, and the `pre-compact` hook; then, in the factory's repository, the labelled channel, the checkpoint and the pause. Automatic checkpoints are on by default only after the pilot of 17.3 passes.
4. **Documents.** `[docs]`, the `docs` commands, the indexes in the per-checkout cache, and a document as a promotion target.

Phases 3 and 4 do not depend on each other.

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

**The owner's decisions.** D1 to D20 of `proposal.md`, all as recommended. Answers of 2026-11-26: a standing rule is offered for banking (6.5); one dialog question settles who and where, and the wording sets the term, with "learned" always long-term, for agents too (6.1, 6.2, 6.4), except that the round 1 fallback for a host with no dialog is replaced, pending the owner's confirmation (section 20); the long-term quota is 500 memories and 16 MiB (7.2); `statement` is the kind of a remembered statement (3.2); `pbrain.toml` stays in the brain folder (11.1); `docs search` orders by match class with items ahead of documents inside one class (11.5).

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

## 20. Open questions

For the owner, each with a recommendation:

1. **Agents' lessons and the long-term quota.** "Learned is always long-term" applies to agents, and a busy team could fill the quota with lessons in days. From then on a person's own "remember" is refused until someone prunes. Promotion does not help by itself: it leaves the original memory in place and frees nothing; only `forget` or a chosen end date does. *Recommended:* keep one quota of 500 memories and 16 MiB, but reserve 200 memories and 6 MiB of it for memories a person asked for, so that agents' lessons can never crowd them out in either measure. Only a requester the adapter observed as a person counts for the reserve: a caller that merely passes `--asked-by human:owner` does not consume it; and have the skill, once a task's lessons are banked and the item is reviewed, offer to forget them. *Alternatives:* a separate, larger quota for what agents learned; or one undivided quota, relying on pruning.
2. **Learned as a mark, or as a kind of its own.** This spec marks a `statement` as learned (3.2), as @architect-2 and @lead-dev both recommend: the mark is separate from how the text was captured, nothing resembles the item kind `learning`, and the always-long-term rule and the offer to bank work the same. *Alternative:* a fifth kind, `lesson`, which @architect first proposed because the rule, recall's order and the promotion target would all hang on one field. Either can be built.
3. **Nothing is stored on a host with no dialog.** In round 1 the owner chose a fallback that stored a typed "remember" for the team as short-term when no dialog exists. The owner's refinement, that the wording sets the term, and the pause rule of D18 both speak against it: the fallback would change the term the person asked for, and would put "remember only for me that X" into team memory, which a later reply cannot undo. This spec therefore stores nothing in that case and gives the person the exact command (6.2). @architect-2 and @lead-dev both call the old fallback a disclosure bug. *The owner should confirm this change to an answer already given.* It does not affect Claude Code, which has a dialog, except when it runs without a terminal.
4. **The dialog on every "remember" typed by a person.** The owner chose it. If one click per request proves irritating in use, the alternative the reviewers refused, trusting the prompt in a session a person started alone, would have to be reopened with them.
5. **Standalone use has no automatic checkpoint.** A checkpoint needs a supervisor to vouch for the context, so a person using pbrain with one agent and no factory gets explicit capture, recall and the task page, and no checkpoint prompt. *Recommended:* accept for the first version.

For the team, to settle in the architecture step: whether the document index is its own or shares one with managed items underneath the separate commands; how the adapter reads the previous answer and the effort from each host; how a supervisor gives a long-lived agent a clean context for its next task; and the byte limits on a section's stored state and on its replay receipts from the reviewed #83 plan, which must be kept, since the counts in section 7 alone do not bound a section that holds 64 KiB texts and many keyed writes.
