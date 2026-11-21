---
kind: source
title: "Tokenizer pack experiment for EmbeddingGemma 2"
created: 2026-11-21T12:00:00Z
author: agent:fixture-editor
source: "pbrain@24879e6 research/semantic-search-and-index/embedding-models-and-go-runtime/goinfer-tokenizer-spike.md"
status: provisional
tags: ["models","tokenizer"]
---

# goinfer fork: the fast-loading tokenizer spike

Run 2026-11-21 by @lead-dev-2. It is the first of the three goinfer changes in `goinfer-architecture-review.md`, the one the plan said to prove first. Measured on an Intel Core Ultra 7 255H, Linux x86_64, Go 1.27.3, `GOMAXPROCS=4`, warm file cache; single runs unless stated.

## Bottom line

**It works.** A prebuilt binary "pack" of EmbeddingGemma 2's tokenizer loads in **24 to 26 ms**, against **441 to 502 ms** for its `tokenizer.json`. It gives **identical token ids**:
- on goinfer's 48 golden texts (the sentence-transformers reference ids);
- on 10,000 seeded random encodings, against the `tokenizer.json` path, both with and without special-token parsing;
- on **3,052 texts and 250,897 tokens** against the Hugging Face `tokenizers` library itself: the 22 parity texts with their prompts, 3,000 seeded random texts mixing scripts, emoji, control-like and special-token text, and the 30 corpus windows.

The pack is **9,581,663 bytes**, against 32,170,510 for `tokenizer.json`. Building it from `tokenizer.json` takes 0.75 to 0.84 s, once, when the model is derived. The tokenizer item of the plan is done, inside the "tens of milliseconds" target. The architect's fallbacks (an encode-only form, or dropping the model) are not needed.

## Where it is

- **Repository:** the owner's fork, `~/Dev/pkieltyka/goinfer` (`github.com/pkieltyka/goinfer`), on the local branch **`pbrain/fast-tokenizer`**, from `main` at `e0d66171`.
- **Commits** (pushed to github.com/pkieltyka/goinfer and merged into the fork's main: PR #1 as `2f80ef14`, then the changelog.d fragment `0cad28e3` as PR #2, `2e413ee1`; offered upstream as townsendmerino/goinfer#1194):

  | Commit | What |
  |---|---|
  | `6f6806d5` | `tokenizer: prebuilt pack form of a SentencePiece-style BPE tokenizer` |
  | `bc477653` | `tokenizer: fill a pack's merge ranks into a flat hash table` |
  | `e98418db` | `fuzz-weekly: run tokenizer FuzzLoadPack` |

- **Remotes:** an `upstream` remote for `github.com/townsendmerino/goinfer` was added for fetching, with its push URL set to `DISABLED`.
- **The repository's rules** (its `CLAUDE.md`, `AGENTS.md` and `CONTRIBUTING.md`):
  - explicit staging;
  - `Co-Authored-By` trailers, with no session or machine names in commit text;
  - comments that say what is true now;
  - numbers in commit messages with their machine and settings;
  - a new fuzz target listed in `fuzz-weekly.yml`, which the repository's own layout test requires.

## What was changed

- **`tokenizer/pack.go`** (new):
  - `BuildPack(tokenizerJSON []byte) ([]byte, error)` and `LoadPack(pack []byte) (*Tokenizer, error)`;
  - the error class `ErrPack`;
  - `flatMerges`, an open-addressing table for the merge ranks.
- **The pack format**, little-endian:
  - magic `GITOKPK1`, then a version;
  - flags: byte fallback, ignore-merges, prepend-space, strip-leading-space;
  - the special-token ids;
  - one string table: every piece by id, then the few strings outside it;
  - vocabulary entries (string index, id);
  - merges as id pairs in list order, with a negative value naming a string, for the rare merge with a piece outside the vocabulary;
  - added tokens with their rstrip flag;
  - a trailing SHA-256 of all of it.
- **`LoadPack` rebuilds exactly the `Tokenizer` that `parseTokenizerJSON` builds** for this family:
  - pieces are substrings of one string block;
  - the vocabulary map and the merge table are filled in parallel;
  - the byte-fallback table and the added-token trie are rebuilt as `initGemma` does.
  
  It accepts only the SentencePiece-style byte-fallback BPE family (Gemma); a byte-level or score-ranked tokenizer is refused at `BuildPack`. Every count, offset and id is bounds-checked before anything is allocated, and a malformed pack is an `ErrPack` error, never a panic.
- **`tokenizer/sentencepiece.go`:** `mergeTable` gains an optional `flat` field. `rank` and `len` consult it when it is set. Every other loader is untouched.
- **Determinism:** the same `tokenizer.json` always gives the same pack bytes (`TestBuildPack_deterministic`). The pack built from `google/embeddinggemma-2` revision `f9972ac2`'s `tokenizer.json` has SHA-256 `45a7e7d5a1e84ad83ef469a8f6aecc3dd15f51b7c4866008e2d770142d96b709`, so pbrain can pin it beside the source file's digest.

**Where the load time went:**

| Step | Before | After |
|---|---|---|
| Reading and parsing the JSON | 18 ms + 137 ms | 0 |
| Normalising the 514,906 merges | 165 ms | 0 |
| Merge table (515k entries) | Go map, built with two text lookups per merge | flat table filled by id, about 50 ms faster than a pre-sized Go map |
| Vocabulary map (262,144 entries) | from JSON | about 25 ms, in parallel with the merges |
| Pack checksum | | about 3 ms |
| **Total** | **441 to 502 ms** | **24 to 26 ms** (best of 5; 53 to 63 ms with a Go map for the merges) |

**Memory:** the loaded tokenizer keeps about 16.6 MB on the heap, and about 35 MB is allocated in all during the load. `LoadPack` does not keep the pack's bytes.

## How it was checked

All of these ran in the fork, on the branch:
- **Tier-0 unit tests** (`tokenizer/pack_test.go`):
  - `TestPack_equalsJSON`: a synthetic Gemma-style tokenizer with byte fallback, a merge with a piece outside the vocabulary, a merge listed twice, and added tokens with rstrip. The pack and JSON tokenizers' tables match field by field, every merge rank matches and so does every absence, and 2,000 random texts encode and decode the same in both special-token modes.
  - `TestBuildPack_deterministic`, `TestBuildPack_refusesOtherFamilies`.
  - `TestLoadPack_refusesDamage`: truncations, bit flips, and structural damage behind a valid checksum (huge counts, offsets out of order, a bad version, an id out of range, trailing bytes).
- **Fuzzing:** `FuzzLoadPack` ran 35 s in two runs, more than 13 million inputs, with no failure. It is listed in `.github/workflows/fuzz-weekly.yml`.
- **Real checkpoint** (`tokenizer/pack_real_test.go`, build tag `realckpt`, `GOINFER_EG2_DIR`): the 48 golden texts and the 10,000 random encodings above, and the timings.
- **Against Hugging Face `tokenizers`:** a scratch program in my scratch folder, outside the tree, so nothing new is committed for it.
- **The repository's checks:**
  - `python3 scripts/touched.py`: **VERDICT PASS, 17 checks**. Its first run failed on the layout test until `FuzzLoadPack` was listed.
  - `go run ./cmd/lint` with the pinned staticcheck 2026.2.1 (0.8.1): **VERDICT PASS, 7/7**.
  - `gofmt -l`: clean.
  - In this sandbox the timing lock (`GOINFER_TIMING_LOCK_FILE`) and staticcheck's cache were pointed into my scratch folder, because the home directory is read-only here. On the maintainer's box they use their defaults.

## How it plugs into the bytes loader

The architecture's EmbeddingGemma 2 adapter (`components/semantic.md` at 9e1ddcc3, "The EmbeddingGemma 2 adapter") derives a model pack from the four pinned source files and opens the model only from verified bytes.
- **Derive.** Call `tokenizer.BuildPack(tokenizer.json bytes)` and store the result as the pack's tokenizer file. Its digest is fixed for a pinned source, so the release table can pin it as the architecture requires.
- **Open.** Call `tokenizer.LoadPack(artifact bytes)` to get a `*tokenizer.Tokenizer` directly, opening no file.
- **One gap remains,** for the bytes-loader change. `embeddinggemma2.LoadEncoder(dir)` reads `tokenizer.json` by path today, and `Encoder`'s fields are private. The loader change needs a constructor such as `embeddinggemma2.NewEncoder(m *Model, tok *tokenizer.Tokenizer, prompts map[string]string) (*Encoder, error)`, beside @architect's `Model`-from-bytes loader. It is a small addition, in the same branch family.
- **Special tokens** behave as before: a pack-loaded tokenizer serves `Encode` and `EncodeSegments` unchanged. So the adapter's rule that a literal `<eos>` in a body never becomes a special token is met with `EncodeSegments` and `Special: false`, as for the JSON-loaded tokenizer.
- **Two checksums.** `LoadPack`'s own checksum (about 3 ms) repeats the Artifact's digest check. That is cheap and keeps the function safe for other callers.

**Effect on @architect's estimate.** He put goinfer's verify, load and one query at 360 to 450 ms once all three changes land, with the tokenizer taking "tens of milliseconds". The measured 24 to 26 ms is at the bottom of that, so his figure stands on this point.
