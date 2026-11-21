# EmbeddingGemma 2 in pure Go: goinfer, rembed and the interim runtime

A joint note by @architect and @architect-2, 2026-11-21, for the owner. It rests on three reports: [goinfer-embeddinggemma2.md](./goinfer-embeddinggemma2.md) (@lead-dev-2's independent check), [goinfer-architecture-review.md](./goinfer-architecture-review.md) (@architect, with a prototype of the load fix) and [rembed-embeddinggemma2.md](./rembed-embeddinggemma2.md) (@architect-2). The interim runtime's numbers are from [spike-bge-small-hugot.md](./spike-bge-small-hugot.md).

Every number is from one Intel laptop at 4 threads with a warm file cache, a few runs each, and the machine was shared during the later runs. Nothing was run on arm64 or macOS.

## Recommendation

1. **Keep bge-small-en-v1.5 on hugot as the interim runtime.** It is the only candidate with demonstrated reference parity, loading from verified bytes and a first query within 600 ms. It also had the lowest memory use and led on the 20 questions. The held-out retrieval gates and a Mac measurement remain before release.
2. **Adopt neither EmbeddingGemma 2 runtime now.** Both are correct on ordinary text and pure Go. Both load in over a second and read the model from a folder, so neither meets the 600 ms rule or the rule that the bytes checked are the bytes loaded.
3. **If EmbeddingGemma 2 is wanted, build on goinfer.** It has a tagged release, it matches the reference on all 22 texts, and a prototype has already shown most of its load time can be removed. rembed's EmbeddingGemma 2 code exists only in an untagged fork three commits ahead of its upstream.
4. **Decide on the model before paying for a runtime.** Use goinfer as published to produce EmbeddingGemma 2's vectors for the evaluation fixture. Two questions are answered there, and they are different:
   - the architecture's gates: does the model find more than lexical search alone, and does it abstain on held-out questions the brain cannot answer?
   - the adoption question, which is ours: is it better than bge-small by enough to pay for the runtime work, a first index eight to ten times longer and more memory? On our 20 reworded questions it was not: 9 right against 13.
5. **The owner does not need to write a library from scratch.** If the fixture favours EmbeddingGemma 2, the owner's library is goinfer's text encoder with three changes, offered upstream first and kept as a pinned fork otherwise. If it does not, no library is needed.

The Embedder seam and the rules of architecture 8.7 stay as they are.

## Side by side

| | bge-small-en-v1.5 on hugot (interim) | EmbeddingGemma 2 on goinfer | EmbeddingGemma 2 on rembed |
|---|---|---|---|
| Source and version | `knights-analytics/hugot` v0.8.6, tagged; Apache-2.0 | `townsendmerino/goinfer` v0.24.1, tagged; MIT | fork `ex1tium/rembed` at commit `f401a554`, untagged; upstream v0.3.0 lacks the model; Apache-2.0 |
| Pure Go, four targets, no shared library | yes | yes | yes |
| Binary size added | 18 to 24 MB | about 1 MB | 4.3 to 4.9 MB |
| Matches the reference (cosine at least 0.999 per text) | 22 of 22 | 22 of 22 | 21 of 22: a literal `<eos>` is read as text, cosine 0.952 |
| Builds from bytes already checked in memory | yes, through a small shim | no: loads from a folder path | no: loads from a folder path |
| Load and first query | 440 to 485 ms, including verification | About 1.4 s before verification (load 1.26–1.39 s, first query 90–120 ms) | 1.10 to 1.13 s before verification |
| Read and hash the model's files | about 110 ms for 134 MB | the published file is 1.49 GB: 1.2 to 1.6 s | the same file: 1.2 to 1.6 s |
| Warm query | 23 to 27 ms | 81 to 91 ms | 34 to 49 ms |
| Indexing speed | about 4,100 tokens a second | about 440 | about 520 |
| First index of 5,000 items | about 10 minutes | about 95 minutes | about 80 minutes |
| Long inputs | cut at 512 tokens | 2,294 tokens in 5.8 s | 2,294 tokens in 83 s |
| Peak memory at a search | 0.60 GB | 1.87 GB | 1.62 GB |
| Right section first, and in the top three, of 20 questions | 13 and 17 | 9 and 12 | 9 and 12 |
| Reads the environment or prints | yes: needs the `envguard` package and the null-device rule | no | not on the local path; its other loader reads tokens and downloads, and must not be reachable |
| Maintenance | fast-moving v0.8; one internal branch relied on | one author, hundreds of commits a day, a second repository for the kernels | one upstream author; the model lives in a three-commit fork by another person |

The first-index estimates assume roughly 2.5 million tokens across 5,000 items. The throughput tests used the same 30 corpus windows. Hugot used batches of eight and truncated each input at 512 tokens. The two EmbeddingGemma 2 runs used one untruncated input per call.

## What the two EmbeddingGemma 2 runtimes share

These costs come with the model and its published file, whichever library runs it:

- **One 1.49 GB multimodal file.** The text part is 542 MB. Reading and hashing the whole file took 1.2 to 1.6 s, twice the budget by itself. A text-only pack, derived once by `pbrain index` from the pinned download and pinned by its own digests, took about 140 ms to read and hash with four readers.
- **A 30 MB tokenizer file** that costs about half a second to parse in both libraries.
- **A slow first index**, 80 to 95 minutes for 5,000 items, because the forward pass runs at 440 to 520 tokens a second.
- **The same retrieval result**, 9 of 20, since both produce the model's own vectors.

## Why goinfer and not rembed as the base

- **Parity.** goinfer matches the reference on every text. rembed's one failure is its tokenizer; fed the reference's token ids, the same encoder matches. It would need a tokenizer change either way.
- **A measured way out of the load time.** In goinfer's prototype, keeping the token table in its stored 16-bit form and widening the other tensors once in parallel brought the weights from about 1,000 ms to about 120 ms with identical vectors. With a fast tokenizer and the text-only pack, the estimate is 360 to 450 ms for check, load and first query; the tokenizer part of that is not built. rembed already keeps its token table in 16-bit form and still spends about 780 ms on the weights, so its gap is elsewhere and unexplored.
- **Long inputs.** rembed scores every pair of tokens before masking, which made a 2,294-token text take 83 s against 5.8 s. pbrain's passages are short, so this matters less than it looks, but it is a sign of where each code base stands.
- **Supply chain.** goinfer has releases and one author. rembed's model code is an untagged fork by a second person, whose module still carries the upstream's name, so pbrain would pin a commit of a fork of a project.

rembed is worth keeping as a reference for two things it does better: short queries (34 to 49 ms against 81 to 91) and a compact token table.

## What would change this

- EmbeddingGemma 2 clearly ahead of bge-small on the fixture: then the goinfer work is worth doing, in the order of its review, the fast tokenizer first.
- A release of either library that loads from bytes in memory within the budget: then it can be measured against the rules as it stands.
- Results on a Mac that differ from these: all of the above is one Intel laptop.

## Not checked

- Any arm64 or macOS run of any of the three.
- A cold file cache.
- The goinfer estimate end to end: its tokenizer change is not built.
- The fixture itself: the 20 questions are answerable, English, and written by one person.

**Sign-off.** @architect-2 checked the comparison against the four reports and agrees with the recommendation.
