# Document quality fixture

This folder is part of the pbrain-2 functional spec (section 17.4). It is frozen: the spec names the SHA-256 of this file and of `questions.json`, and a change to either, or to any file listed here, is a change to the spec and needs the same review.

It was written on 2026-11-26, before any document code exists. Nothing here was tuned against an implementation.

## The corpus

Thirteen documents, all in `tree/` in this folder, so that a test needs no Git history to build the corpus. `tree/docs/` is the enrolled document folder. The managed area beside it is the existing retrieval fixture's brain (`internal/semantic/calibrate/testdata/retrieval/brain`, 35 items), placed at `docs/brain`.

Six are byte-for-byte copies of committed research documents. The four under `docs/research/` are the long documents: each is over 12,000 characters, and each of its keyword and semantic questions is answered in its last quarter. Seven were written for this fixture.

| File | What it is | Characters | Lines | SHA-256 |
| --- | --- | --- | --- | --- |
| `tree/docs/research/memory-providers-and-hooks.md` | A byte-for-byte copy of `specs/projects/pbrain-2/research/hermes/memory-and-learning-loop/memory-providers-and-hooks.md` as committed in `d9a9c3e` (Git blob `0da932ae1cd532791ed877728d8c42efdff99559`) | 16,811 | 130 | `270bcf68653ee1699f4124eba08480c86a536ee41ba8855af1c0956770bd69cf` |
| `tree/docs/research/failure-modes-from-the-tracker.md` | A byte-for-byte copy of `specs/projects/pbrain-2/research/hermes/evidence-and-reports/failure-modes-from-the-tracker.md` as committed in `d9a9c3e` (Git blob `e5dd89a6ddbdef06ee6f0cb4012bcc2930580745`) | 16,428 | 62 | `afaa04dfe4cb44d010a70f303c11220e640c1695b0ce322318433f8bb59d510b` |
| `tree/docs/research/borrow-and-refuse.md` | A byte-for-byte copy of `specs/projects/pbrain-2/research/hermes/lessons-for-pbrain-2/borrow-and-refuse.md` as committed in `d9a9c3e` (Git blob `61b5fdde96475895cd7997e3e358466cf13363db`) | 13,844 | 88 | `26cd18d598a6f6741aad9a5c9538e170be736ca77a442897a4a2b15402d7cd4f` |
| `tree/docs/research/learning-loop-background-review.md` | A byte-for-byte copy of `specs/projects/pbrain-2/research/hermes/memory-and-learning-loop/learning-loop-background-review.md` as committed in `d9a9c3e` (Git blob `4fd30fa6b21181015174139c5aba94524a1f1e2e`) | 13,704 | 88 | `eabfc9a67f00638a9a80d9cdefb7b6993e5b2bb8a454209ce6547e21af8e4687` |
| `tree/docs/skills/research_summary.md` | A byte-for-byte copy of `specs/projects/pbrain-2/research/hermes/skill-creation/research_summary.md` as committed in `d9a9c3e` (Git blob `050e47258064837af64fca71274975ff43ded00a`) | 11,786 | 51 | `c5f062b2b198599c92755240d18a64c9f9aef9a3ff9339aaf49909c8e043d5cf` |
| `tree/docs/evidence/research_summary.md` | A byte-for-byte copy of `specs/projects/pbrain-2/research/hermes/evidence-and-reports/research_summary.md` as committed in `d9a9c3e` (Git blob `6197573846505c0309ed91aaa1b077e20dfc42f1`) | 12,530 | 42 | `0de5fe1b2a611288a55c6d6a21732b1f0d52bb117a1a88021561d0a2f0f3bb12` |
| `tree/docs/site/README.md` | Written for the fixture. One of two files named README.md | 243 | 5 | `c00e437a5996e4366663c6dbce0b6312a9a2b6a85504b130d130696bad175eee` |
| `tree/docs/ops/README.md` | Written for the fixture. The other README.md | 220 | 5 | `ca6b993f43047b2acc4eefcdd7afd608261a96db433bc837f009dc0530c2283c` |
| `tree/docs/site/preview-deployments.md` | Written for the fixture. A foreign header with a title; no heading in the body | 491 | 12 | `f0e25e208a501c2e6624b0eeaaf55d79f180c00c857ac7f0b39d840d203b8919` |
| `tree/docs/ops/broken-header.md` | Written for the fixture. A header that is never closed | 338 | 8 | `7a76ec0b7aea6d11e3a3f19b982c117fbddd7f22174ee52e3c5fc3614f5289c2` |
| `tree/docs/ops/imitates-standing.md` | Written for the fixture. A header that imitates an item's standing | 440 | 11 | `a57084f03483ec7edb56b99a2c3f46da356415316b2db196587faf88e8558dff` |
| `tree/docs/ops/index-refresh-notes.md` | Written for the fixture. Shares words with the fixture item index-refresh and answers none of its questions | 292 | 5 | `457f850a6ac75f24eaac088d03a2f50eca718ddbdf155ba8296988cd407c411d` |
| `tree/docs/ops/delete-preview-notes.md` | Written for the fixture. Shares words with the fixture item delete-preview and answers none of its questions | 315 | 5 | `4082fad3b63feed3a1f01f212fb47eebcffd1c4a5633c11c9c631a9fbb4cb7f2` |

None of the thirteen has pbrain front matter. The two files named `research_summary.md` and the two named `README.md` are the equal-name pairs.

**To run the gate:** a scratch Git project with `tree/docs` as `docs/`, the 35 items as `docs/brain`, `[docs] folders = ["docs"]`, the calibration tool's fixed clock, the shared section, and the release's default model and budgets. Check the thirteen hashes above before anything else.

## The questions

`questions.json` holds 32 entries.

| Kind | Count | How it is asked | What must hold |
| --- | --- | --- | --- |
| `keyword` | 14 | `pbrain docs search <q>`, with no flag | Every path in `expect` is among the first `rank` results. For each, the result's location line is exactly `location_line` and its body offset exactly `snippet_offset` |
| `keyword-nothing` | 2 | the same | No result at all |
| `question` | 12 | `pbrain docs search --semantic <q>` | Every path in `expect` is among the first `rank` results (5). The passage whose start the result's location names contains `evidence_lines`: the location is at or before the first of them, and that passage runs to at least the last |
| `unanswerable` | 4 | `pbrain docs search --semantic <q>` | No result at all |

**Two different lines.** A result's location line says where its snippet starts, which is not the line that holds the answer. So each entry keeps them apart:

- `evidence_lines` is where the answer is: the file lines that hold the query's words, or the passage that answers the question.
- `location_line` and `snippet_offset`, on a keyword entry, are where the snippet must start under the snippet rule of the pbrain spec (section 7.1): the earliest stretch of 200 characters that holds the most distinct query words, and the start of the body when the body is shorter than that or holds none of them. A document's body is the whole file when it has no well-formed header, and otherwise what follows the header's closing line and the one empty line after it.

**How the keyword entries were computed and checked**, on the day of writing:

- A script that splits text into words as pbrain does (runs of letters and digits, case ignored) confirmed that every word of each keyword query is in each expected document, on the stated evidence lines, with one exception that is intended: `k11` matches only in the title taken from a foreign header, so it has no evidence line in the body.
- `location_line` and `snippet_offset` were computed with the snippet rule, and the offsets were then compared with the product itself: each document body was stored as an item in a scratch brain and searched with the binary built from commit `d9a9c3e`. All thirteen offsets that have a body match agreed.
- No fixture item holds all the words of any keyword query; the two `keyword-nothing` words occur nowhere that is searched; and the invented names in the four unanswerable questions occur in no document and no item.

**What was not checked.** The twelve semantic entries: no model was run. Their `evidence_lines` were read from the documents. If one fails, that is a finding about the implementation or the model's cut-off to bring to the owner, not a reason to edit this file quietly.

## The existing questions

The retrieval fixture's own `questions.json` (SHA-256 `af4054bbb01300f9f3097b3b414f4a4a5f05d246fa62b2cd48ea906358b5c041`, 47 answerable and 60 unanswerable questions) is part of the gate unchanged. Each is asked twice, plainly and with `--semantic`, first through `search` on the 35 items alone, which gives the legacy outcome, and then through `docs search` with the thirteen documents enrolled. The spec (section 17.4) says what must be equal. The two documents `docs/ops/index-refresh-notes.md` and `docs/ops/delete-preview-notes.md` exist to tempt it: they share words with the items `index-refresh` and `delete-preview`.
