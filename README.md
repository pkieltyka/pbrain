# pbrain

pbrain keeps project knowledge in Git as Markdown. People and agents can record decisions,
learnings and sources, search for a few relevant items, and cite them by ID. By default,
search returns up to five results in 3,000 characters. Shared items live in the repository.
A Git-ignored personal section is available with `--personal`.

## Install

With Go 1.27 or later, install from a pbrain checkout.

```sh
go install ./cmd/pbrain
```

While the repository is private, tagged installs and release downloads require repository access.
Once a release is tagged, install it with
`go install github.com/pkieltyka/pbrain/cmd/pbrain@latest` or download the
Linux or macOS binary for your architecture (amd64 or arm64). [GitHub Releases](https://github.com/pkieltyka/pbrain/releases)

## Quick start

From the root of your project, create a brain in `docs/brain` and record a decision.
Use your own name for the author and the person who made the decision.

```sh
pbrain init
pbrain add --kind decision --title 'Sessions live in Redis' \
  --body 'A shared store keeps sessions available across API restarts.' \
  --author human:alex --decided human:alex
pbrain search sessions Redis
```

Copy the ID from the search result into `pbrain get <id>` to read the decision.
Plain search reads the Markdown directly, so a new item is searchable immediately.
Use `--json` for machine-readable results. Run `pbrain skill` for the agent guide
or `pbrain skill list` for its topics.

## Semantic search

Use `pbrain search --semantic 'where is login state kept'` to find related wording.
Exact hits come first. When the model or index is unavailable, search returns
the lexical results and a notice. Search never downloads a model or builds an index.

**The first index on a machine is a person's step.** Check `pbrain index --status`,
then run `pbrain index` when ready. It downloads the selected model if needed and
embeds items locally. A small brain takes minutes. With the default model on four CPUs
the build ran at 0.902 passages a second (a passage there averaged about 1,000 characters): about
15 hours for the 5,000-item, 50 MB test brain, with a peak of about 2 GB of memory. That
generated text is token-heavy, so ordinary notes may go faster. You can stop the build.
The next `pbrain index` continues from where it was.
After writes or a pull, rerun `pbrain index` to pick up changes.

The default model is `embeddinggemma-2`. To use the smaller, faster
`bge-small-en-v1.5`, set the model in `docs/brain/pbrain.toml` and run `pbrain index` again.

```toml
[semantic]
model = "bge-small-en-v1.5"
```

Set `model = "none"` in the same section to turn semantic search off.

| Model | First download | Local model files | Cutoff | Expected item on first page (47 answerable) | Empty page (30 unanswerable) |
| --- | ---: | ---: | ---: | ---: | ---: |
| `embeddinggemma-2` | 1.52 GB | 552 MB | 0.72 | 42 | 30 |
| `bge-small-en-v1.5` | 134 MB | 134 MB | 0.65 | 40 | 29 |

The download notice rounds up to whole MB (1,522 MB for Gemma, 135 MB for bge).
Allow about 2.1 GB on disk for Gemma's model files during the first build, while the
download and the derived local model coexist. The index needs additional space.

On this fixture, strict search found the expected item on 16 of 47 first pages.
Both models kept all 16, and adding `--bm25` to `--semantic` gave the same counts
shown above. For six long-document questions, the reader saw the supporting passage
on the search page or after one follow-up read in five cases with Gemma and four
with bge. [Calibration results and method](internal/semantic/calibrate/README.md)

## Measured speed

| Release measurement (5,000 items, 50 MB) | Linux | Apple silicon Mac |
| --- | --- | --- |
| Search and get | Pending | Pending |
| Semantic search, first index and one-note update, per model | Pending | Pending |
