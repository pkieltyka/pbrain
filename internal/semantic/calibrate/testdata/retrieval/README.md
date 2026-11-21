# Retrieval fixture

Use this corpus and question set together for architecture §14 and the calibration procedure in `components/semantic.md`. There are 47 answerable questions and 60 questions that have no answer in the shared brain. The latter are assigned to two fixed halves of 30. All question IDs and item IDs are stable.

## Materialize a run

`brain/` contains the searchable shared Markdown. `personal-overlay/` contains eight invented personal notes for the disclosure check. Keep `questions.json`, this README and the overlay seed outside the searchable root. From this directory, prepare an isolated copy outside any Git worktree:

```sh
run_dir=$(mktemp -d)
cp -R brain "$run_dir/brain"
cp questions.json "$run_dir/questions.json"
printf '%s\n' 'semantic = { model = "bge-small-en-v1.5" }' > "$run_dir/brain/pbrain.toml"
```

Pass the run directory to the calibration tool and the brain directory to pbrain's `--brain`. Use the selected release model in the scratch TOML. This setup keeps tracked test data from registering as another project brain. Keep models and per-user caches outside both the checkout and the scratch brain. Freeze the evaluation clock at 2026-11-21T18:00:00Z with TZ=UTC, the same for every arm. Use the released search budget (initially 3,000 code points and five hits). With options preceding the query, pass each `q` as a single operand after `--`, including literal flags such as `--fresh`. Pass the question text itself to the embedding model.

Collect the shared-view pages and diagnostics first. Copy `personal-overlay/` into the scratch brain as `personal/`, build the personal index there for hybrid runs, then replay the shared questions at the same clock and budget. Compare complete output, including counts, positions and notices. With `--personal`, directly reading each control ID below must recover its answer. The scored personal-only questions run in the **shared** view. No real personal data is used.

| Question | Personal item | Question | Personal item |
|---|---|---|---|
| u027 | larkspur-host | u057 | mallow-bench |
| u028 | meridian-slot | u058 | kestrel-export |
| u029 | quartz-checklist | u059 | hazel-binder |
| u030 | sundial-review | u060 | thistle-probe |

## What the labels mean

- `expect` lists current items that directly answer the question. Superseded decisions are real searchable distractors with `--all`. Score the normal current view. Short adopted filenames are valid item IDs under functional spec §2.3.
- `style` separates reworded requests, short work terms, identifiers, superseded answers, long documents, far and near misses, and personal-only answers. Each unanswerable half has 14 near misses, 12 far misses and four personal controls. `regression` singles out required empty results. The Zurich transcript case was already known from the BM25 discussion and is retained as an explicit regression.
- The six `long-document` questions each name exactly one source and quote evidence from its body. Normalize whitespace as the semantic design specifies. The three sources exceed an ordinary get budget. A quoted answer elsewhere in a source does not count as seen unless the JSON search page or the permitted follow-up get actually includes it.
- Compare the attachment-retention question with the exact on-disk verification question, and the startup and exposure questions with their detailed source lookups. Accept a short learning or a source for questions they both answer. For the section-specific questions the quotation determines success.

## Provenance and baseline

The shared items summarize decisions and investigations from pbrain at `24879e6`, the factory team brief and the factory getting-started guide at `b47b1a21`. Each item cites its origin. The long sources are excerpts of the brain and store designs and the tokenizer spike. The three historical pairs represent actual changed decisions on storage, runtime and search budget. Front-matter actors and dates are fixture metadata. Questions are editorial rewrites of engineering and agent-use tasks, not a sample of recorded production traffic. The other unanswerable questions include deliberately invented incidents, absent measurements and unrelated operations requests. An answer must be supported within the supplied view.

For a current-workflow baseline, start a fresh context for each request. Give the reader the same shared files, question text and context allowance. Let it use ordinary filename/text search and targeted file reads. Keep the labels, expected IDs and quotations hidden from the reader. Record its answer or abstention, cited IDs, calls and actual output delivered, including empty searches. Charge follow-up reads and stop at the same allowance across arms. For the pbrain arms use the released JSON budgets and fixed reader in the semantic design. Report reached evidence, costs and misses separately.

Freeze this directory in one reviewed commit before tuning. Use only the calibration half to choose candidate thresholds. Run the held-out set at the frozen settings. If the owner supplies new questions after the freeze, version the fixture and keep the original results with their commit.
