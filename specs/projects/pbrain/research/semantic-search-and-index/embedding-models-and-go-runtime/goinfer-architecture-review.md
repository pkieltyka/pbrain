# goinfer for EmbeddingGemma 2: an architecture review

Written 2026-11-21 by @architect, at the owner's request. It builds on @lead-dev-2's independent check ([goinfer-embeddinggemma2.md](./goinfer-embeddinggemma2.md)) and asks one question: what would it take for goinfer to meet the acceptance rules of architecture 8.7, above all checking the model's files, loading it and answering one query within 600 ms.

How it was done:

- I read the source of `github.com/townsendmerino/goinfer` v0.24.1 (`embeddinggemma2/model.go`, `encoder.go`, `accel.go`, `tokenizer/sentencepiece.go`) and of its dependency `github.com/townsendmerino/aikit` v1.76.0 (`embed/safetensors.go`, the `linalg` kernels' entry points) before running anything.
- I built a scratch prototype of the load fix and ran it against the real checkpoint (`google/embeddinggemma-2`, the copy @lead-dev-2 downloaded). Nothing was installed and nothing outside the scratch folder was written.
- **The machine was busy.** Other agents were benchmarking on it (load average 9 to 10), so my absolute times are 15 to 60% higher than @lead-dev-2's quiet runs. The unchanged library is measured in the same runs, so the ratios hold. Every number is a handful of runs on one Intel laptop at 4 threads with a warm file cache. "Estimate" marks what I did not measure.

## Bottom line

**goinfer can be brought inside the 600 ms rule, but it takes three changes and a repackaged model, not one fix.** With all of them my estimate is 360 to 450 ms at 4 threads, against 1.4 to 1.7 s today. The parts I could build are measured; the last one is an estimate.

- **The weights fix works and is small.** Keeping the token-embedding table in its stored 16-bit form and widening the other tensors once, in parallel, with no second copy, cuts the weights' load from about 1.0 s to about 0.12 s. On the four texts compared, the vectors are exactly those of the unchanged library.
- **It is not enough.** Two costs remain that the earlier report did not separate:
  - the tokenizer: parsing the 30 MB `tokenizer.json` takes about 0.5 s, a third of today's load;
  - the hashed-bytes rule: the published file is 1.49 GB, which cannot be read and hashed inside the budget.
- **The Artifact rule needs a text-only model pack.** The text tower is 542 MB of the 1.49 GB. Read by four readers and hashed in eight segments, 542 MB costs about 140 ms. So the rule holds if pbrain derives a text-only pack from the pinned download once, with its own pinned digests.
- **Indexing speed is untouched by any of this:** about 440 tokens a second, so a first index of 5,000 items takes about 95 minutes, against about 10 for the accepted interim runtime.
- **goinfer cannot be used as published.** Its loaders take a folder path and read from disk; the fixes need changes inside two packages whose fields are private. That means a fork, or changes accepted upstream.

So goinfer is a sound base for an EmbeddingGemma 2 runtime and not a drop-in. I estimate the work at about a week for someone who knows the code; that is a guess, not a plan. It also needs the fixture's verdict on the model itself, which was the weakest on our 20 questions with every runtime.

## 1. Where the load time goes today

`LoadEncoder(dir)` does two things in sequence.

**The weights** (`model.go`, `Load`). Every tensor of the text tower is read with `TensorF32`, which for the stored bf16 allocates a float32 slice and widens the whole tensor in one scalar loop (aikit `safetensors.go`, `BFloat16sToF32`). `Load` then copies that slice again with `append([]float32(nil), v...)`. The copy is needed only for a float32 tensor, which aliases the mapped file; for bf16 it duplicates a slice that is already private. All 413 tensors go through this on one thread: 271 million values widened, and 1.08 GB written twice.

**The tokenizer** (`tokenizer.Load`). `tokenizer.json` is 30 MB of JSON with 262,144 vocabulary entries and 1,029,812 merges. Measured apart: decoding the vocabulary takes 130 to 145 ms and decoding the merges about 255 ms; building the tables takes the rest of about 500 ms. goinfer does not read the smaller `tokenizer.model`.

The checkpoint's real sizes, read from the file's header:

| Part of `model.safetensors` | Bytes | Share |
|---|---|---|
| `language_model.*` (413 tensors, all bf16) | 542,005,296 | 36% |
| of which the token-embedding table (262,144 × 512) | 268,435,456 | |
| of which everything else in the text tower | 273,569,840 | |
| audio tower | 609,650,176 | 41% |
| vision tower | 334,729,216 | 22% |
| whole file | 1,488,915,288 | |

Half of the text tower is one table of which a query uses about twenty rows.

## 2. The prototype

A scratch copy of `model.go` with three edits, and a new loader of about 100 lines:

1. The model is built from bytes the caller already holds, through aikit's `OpenSafetensorsFromFS` and a file system that hands back the caller's slice without copying.
2. The token-embedding table stays a `[]uint16` view of those bytes (aikit's `Tensor.Uint16s`, written for exactly this), and `EmbedTokens` widens one row per token.
3. The other 412 tensors are widened once, by as many workers as there are threads, with no second copy.

The tokenizer was loaded from the same bytes with `tokenizer.LoadJSONBytes`, in parallel with the weights. The pack was a text-only safetensors file written from the real checkpoint (542,056,416 bytes).

| Step, 4 threads, busy machine | goinfer v0.24.1 as published | Prototype |
|---|---|---|
| Read the model bytes into the process's own memory | (memory-mapped, not read) | 245 to 320 ms with one reader; 87 ms with four |
| SHA-256 of what was read | (none) | 170 to 240 ms in one stream; 54 to 73 ms in eight segments |
| Weights ready | about 1,000 ms (inside the load) | 97 to 137 ms |
| Tokenizer ready | about 500 ms (inside the load) | 500 to 550 ms, unchanged |
| Load, as the caller sees it | 1,450 to 1,600 ms | 500 to 550 ms: the tokenizer, with the weights done beside it |
| First query | 106 to 162 ms | 113 to 170 ms |
| Read, hash, load and first query together | 1,556 to 1,731 ms, with no check of the bytes | 948 to 1,042 ms with one reader and eight hash segments |
| Peak memory | 1.87 GB | 1.43 GB |
| Vectors against the published library | | identical: 1 − cosine is 0 on three queries and one document |

@lead-dev-2's quiet runs of the published library gave 1.26 to 1.39 s to load and 81 to 91 ms a query, so a quiet machine should give the prototype proportionally less.

Two more measurements bound what is left:

- Reading only the 274 MB that is not the embedding table, with four readers, and hashing it in eight segments: 66 to 69 ms.
- A compact binary copy of the vocabulary (2.8 MB) rebuilt into a map: 39 to 67 ms, against 130 to 145 ms to decode the same from JSON. The merges would need the same treatment; I did not build it, because the tokenizer's tables are private to its package.

## 3. What it takes to meet the 600 ms rule

| Change | Where | Effect | Status |
|---|---|---|---|
| Build from bytes, not from a folder | goinfer `embeddinggemma2`: `Load`, `LoadConfig`, `LoadEncoder` read `config.json`, the weights and the prompt table by path | Needed for the Artifact rule at all | Prototyped for the weights; trivial for the two small files |
| Keep the embedding table as stored; widen the rest once, in parallel | goinfer `embeddinggemma2/model.go` | Weights: about 1,000 ms to about 120 ms; 0.4 GB less memory | Prototyped and measured; vectors identical |
| A tokenizer that loads in tens of milliseconds | goinfer `tokenizer`: a prebuilt binary form of the vocabulary and merge table, or a faster reader | Tokenizer: about 500 ms to an estimated 50 to 100 ms | Not built. The vocabulary half is measured (39 to 67 ms) |
| A text-only model pack with pinned digests, read by several readers and hashed in segments | pbrain: `embed.Verify` and the download step | Check of the bytes: impossible on 1.49 GB, about 140 ms on 542 MB | Measured on a pack I wrote by hand |

The budget with all four, at 4 threads:

| Step | Time | Basis |
|---|---|---|
| Read and hash the pack (542 MB) | 140 ms | measured, busy machine |
| Weights and tokenizer, side by side | 100 to 140 ms | weights measured; tokenizer estimated |
| First query | 80 to 170 ms | 81 to 91 ms quiet (@lead-dev-2), 113 to 170 ms busy (mine) |
| **Total** | **about 360 to 450 ms** | against 600 ms |

Without the tokenizer change the total is about 750 ms on a quiet machine and over 900 ms as I measured it: the rule fails. That change is the one I could not prototype, so it is the item to prove first.

**Going further is possible but not needed for the rule.** The embedding table could be verified in blocks, each with its own pinned digest, so that a search reads and hashes only the blocks its tokens touch: the check drops to about 70 ms and 268 MB of memory are saved. It would change `embed.Artifact` from "all bytes in memory" to "bytes verified as they are read", which is a change to a contract both reviewers argued over; I would not make it unless the measurement on the built command asks for it.

## 4. How this fits pbrain's design

**The Embedder seam is untouched.** Texts and a role in, unit vectors out, a fingerprint and a dimension: goinfer fits behind `embed.Registry` as one more entry.

**The Artifact rule** (architecture 8.7: the bytes hashed are the bytes loaded, in the same process) is where the work is:

- goinfer's public loaders read from a folder path, and the memory map they use is not a private copy, so as published it cannot satisfy the rule. The entry points for a bytes-based load exist one level down, in aikit (`OpenSafetensorsFromFS`) and in the tokenizer (`LoadJSONBytes`), which is why the prototype was short.
- **The pack.** Google publishes one multimodal file. This cost belongs to the model, not to goinfer: any runtime that loads EmbeddingGemma 2 from the published checkpoint under the hashed-bytes rule meets the same 1.49 GB. Two ways to get 542 MB instead:
  - *Derive it locally* (recommended). `pbrain index` downloads the pinned 1.49 GB file once, checks its digest, and writes the text-only pack into the cache by a fixed procedure, so the pack's digests are known in advance and are pinned in the release table too. Every later open checks the pack against those digests. Nothing is hosted by pbrain. The cost is a 1.49 GB download for a 542 MB model, and a derive step in `embed` that the interim model does not need.
  - *Host a text-only file.* Apache-2.0 allows it. It makes the download a third of the size and makes pbrain, or the owner's library, a distributor of model files.
- **Several pinned files, checked in parallel.** The model row already lists files, each with its digest. Splitting the pack into a few files and having `embed.Verify` read and hash them concurrently is an implementation detail inside `embed`; the contract does not change. The interim model gains from it as well.

**The adapter's own duties** (`components/semantic.md`, the adapter contract) carry over with three notes:

- *Special tokens.* `Tokenize` uses `tokenizer.Encode`, which reads a literal `<eos>` in a body as the real token. The adapter must make such text harmless first or use goinfer's `EncodeSegments`, as the bge adapter does for its own special tokens.
- *Long inputs.* goinfer refuses an input over 8,192 tokens; it never cuts. pbrain's passages are at most 1,800 code points, far below that, so the adapter needs only a guard. The contract's "cut, never refuse" is met by cutting to the limit before the call.
- *Prompts.* The checkpoint's document prompt is `title: none | text: `. pbrain embeds the item's title and section titles ahead of each passage. Whether the title goes into that slot or stays in the text is a preparation choice for the adapter's design; it changes vectors, so it belongs to the fingerprint and to the fixture.

**Environment and output.** Neither goinfer nor aikit reads an environment variable or prints (confirmed by grep over the packages this path compiles; it agrees with the earlier audit). The `envguard` lesson from GoMLX does not apply. A binary that links goinfer starts in 3 ms, the same as one that links the GoMLX backend.

**Build.** No cgo, no shared library, about 1 MB of binary, against 18 to 24 MB for hugot. Hand-written assembly for amd64 and arm64 comes with aikit; the arm64 kernels have no pure-Go fallback and nobody here has run them.

## 5. The rules of architecture 8.7, one by one

| Rule | goinfer today | With the changes above |
|---|---|---|
| 1. `CGO_ENABLED=0`, four targets, no shared library | passes | passes |
| 2. Reference vectors reproduced (cosine ≥ 0.999) | passes: worst 1 − cosine 1.4e-12 on 22 texts (@lead-dev-2) | passes: the fix changes no value |
| 3. Check the files, load, one query within 600 ms | fails: 1.4 to 1.7 s, and the bytes are not checked | should pass: about 360 to 450 ms, part estimated |
| 4. The two release gates on the fixture | not run | not run; the model was the weakest on our 20 questions with every runtime |

What the rules do not ask, and matters as much:

- **First index.** About 95 minutes for 5,000 items at 4 threads, against about 10 for bge-small on hugot. The per-user cache spares later work trees, not the first build. Nothing in this review changes it; it is the forward pass, at about 440 tokens a second.
- **Memory.** 1.87 GB today and 1.43 GB in the prototype, against 0.60 GB for the interim runtime at a search. An estimate for a finished fix is 0.9 to 1.1 GB: the pack's 542 MB, the widened layers' 548 MB, less the 274 MB of layer bytes that can be dropped once widened, plus the tokenizer. Running the layers from 16-bit weights would halve the rest, but aikit has no such kernel; its quantised kernels exist and would change the vectors.
- **Each search pays the whole load.** pbrain has no resident process, so 360 to 450 ms of every semantic search would be model start-up, against 440 to 485 ms measured for the interim runtime: about the same.

## 6. Risks

- **A fork of a fast-moving, single-author project.** Three of the four changes are inside goinfer packages with private fields. Upstream moves at hundreds of commits a day with no compatibility promise, and its matmul kernels live in a second repository by the same author. A fork must pin both, keep the parity test, and be re-reviewed on each update. Offering the changes upstream is the better outcome and costs little to try: the embedding-row change is the pattern aikit's own encoder already uses.
- **The tokenizer estimate is the weak point.** If a fast tokenizer load proves harder than a binary dump of two tables, rule 3 fails by 150 ms or more.
- **Not measured:** a quiet machine with the prototype; a cold file cache, where reading 542 MB from disk dominates; any arm64 or macOS run; indexing with the prototype (it should equal the published library, since the forward pass is the same code).
- **The model, not the runtime, may be the wrong choice.** On our 20 reworded questions EmbeddingGemma 2 placed 9 or 10 first with every runtime, bge-small 13 and EmbeddingGemma v1 15. A runtime that meets every rule would still have to pass the gain and abstention gates with a model that has done worst so far.

## 7. Recommendation

1. **Do not adopt goinfer now, and do not change the interim runtime.** It fails rule 3 as published and cannot meet the Artifact rule without code changes.
2. **If the owner wants EmbeddingGemma 2, build on goinfer and do not start a new library.** It is correct, pure Go and small, and the measured fix removes most of the gap. The work, in the order that retires risk:
   1. a fast-loading tokenizer, proved first;
   2. the bytes-based loader with the 16-bit embedding table, as prototyped;
   3. the text-only pack, derived locally, with pinned digests and a parallel check;
   4. the adapter (special tokens, the cut guard, the prompt choice) and its contract tests;
   5. measurements on a quiet machine, on a Mac and with a cold cache.
3. **Run the fixture before paying for any of it.** It needs only correct vectors, and the published goinfer already produces them, slowly. Two different questions are answered there and should not be mixed:
   - *The architecture's gates* (8.7): does the model, with its calibrated cut-off, find more than lexical search alone, and does it abstain on the held-out questions the brain cannot answer? A model must pass both to ship at all.
   - *The adoption question*, which is ours and not a rule: on the same fixture, is EmbeddingGemma 2 better than bge-small by enough to justify the work above, the longer first index and the larger memory? If it is not, the runtime work buys nothing, even if both gates pass.

## Sources

- Code read: goinfer v0.24.1 (module sum `h1:JGA9cJpT+AGlR6zEcKr7sm6cqXKMWMj1PCCz8Tp6lPQ=`): `embeddinggemma2/model.go` (`Load`, `EmbedTokens`, the forward pass), `encoder.go`, `accel.go`, `tokenizer/sentencepiece.go` (`Load`, `LoadJSONBytes`, `parseTokenizerJSON`). aikit v1.76.0 (`h1:bQ0KZqOZTGMAFZG/FR+LNjzu0VwxOvQZM1ZRbQ4QVnQ=`): `embed/safetensors.go` (`OpenSafetensorsMmap`, `OpenSafetensorsFromFS`, `TensorF32`, `BFloat16sToF32`, `Uint16s`, `SubF32`), the `linalg` matmul entry points.
- Model: `google/embeddinggemma-2` as downloaded by @lead-dev-2 (`model.safetensors`, 1,488,915,288 bytes); tensor sizes from its header.
- Runs, 2026-11-21, Go 1.27.3, linux/amd64, `CGO_ENABLED=0`, `GOMAXPROCS=4`: a probe with three modes (the published loader, the prototype loader with and without hashing, and a pack writer) and a second probe for read, hash and tokenizer-decode timings. All of it is in this session's scratch folder, not committed.
- [goinfer-embeddinggemma2.md](./goinfer-embeddinggemma2.md) for parity, quiet-machine timings, builds and the safety review; [spike-bge-small-hugot.md](./spike-bge-small-hugot.md) for the interim runtime's numbers.
