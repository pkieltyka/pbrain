# rembed and EmbeddingGemma 2

**Keep bge-small/hugot as the interim runtime.** The Go implementation of EmbeddingGemma 2 is in the `ex1tium/rembed` fork. It cross-compiles for all four pbrain targets and closely reproduces the reference on ordinary inputs. One document containing a literal special token fails the parity gate. The public loader also requires filesystem paths, and a four-thread search exceeds the 600 ms allowance before it verifies the weights. The fork is worth consulting for its compact token table and inference kernels. [Fork at the tested commit](https://github.com/ex1tium/rembed/tree/f401a5543612a4b74ff6baac4cac1695b2cb6876)

## Which project

| Field | Verified on 2026-11-20, Toronto |
|---|---|
| Repository and pin | `github.com/ex1tium/rembed`, commit `f401a5543612a4b74ff6baac4cac1695b2cb6876`. |
| Go module and import | The fork still declares `github.com/rostamlabs/rembed`. I imported that path and used a scratch `replace` pointing at the exact fork checkout. |
| People and release history | Vahid Sohrabloo authored the upstream engine. Git attributes the three fork commits to `ex1tium` on 2026-11-01. The fork tip is three commits ahead of upstream `19d673b3`. Both published tags, v0.1.0 and v0.3.0, predate this implementation. Pinning v0.3.0 or selecting upstream `main` loses the v2 code. |
| Licence and dependencies | Apache-2.0 for the library and checkpoint. The module declares Go 1.26.1 and two dependencies, `golang.org/x/sys v0.47.0` and `golang.org/x/text v0.41.0`. The downstream probe used Go 1.27.3 and verified the module checksums. |

The Go proxy resolves the fork commit to `v0.3.1-0.20261101183610-f401a5543612`, module checksum `h1:vPhQ61yV8flkUljIAhx8MhDHQJLlokaUWc4ycotCGI0=`. The fork added an EmbeddingGemma 2 implementation, model-specific tests and reference fixtures, then corrected its attention workspace and configuration checks. This is useful test coverage, though pbrain would depend on an untagged patch maintained separately from upstream. [Fork commits](https://github.com/rostamlabs/rembed/compare/main...ex1tium:main) · [Module](https://github.com/ex1tium/rembed/blob/f401a5543612a4b74ff6baac4cac1695b2cb6876/go.mod) · [Google checkpoint](https://huggingface.co/google/embeddinggemma-2/tree/f9972ac2)

## What gets embedded

I used Google's bf16 checkpoint at `f9972ac2` with rembed's default floating-point inference mode. The verified safetensors SHA-256 was `197a32965d4b1105faf060417baa899e193fb73cd401f42ec9295234d5553d79`. The adapter supplied the checkpoint's role prefixes, `title: none | text: ` for documents and `task: search result | query: ` for queries. Public `Embed` takes already-prepared strings. The library reads the nested text configuration, frames the byte-fallback BPE tokens with BOS and EOS, and runs the bidirectional Gemma 4-style text tower with per-layer inputs and sliding/full attention. It averages the token representations, applies the bias-free 512→768 projection and L2-normalises. Moving this linear projection after the mean saves work. The matched vectors test that equivalence in practice. [Public API](https://github.com/ex1tium/rembed/blob/f401a5543612a4b74ff6baac4cac1695b2cb6876/rembed.go) · [Text encoder](https://github.com/ex1tium/rembed/blob/f401a5543612a4b74ff6baac4cac1695b2cb6876/internal/model/gemma2emb.go)

The loader maps the multimodal safetensors file, selects the text tensor names and skips the audio and vision towers. It copies the token table into bf16 storage, expands a row on lookup, and materialises the other text weights. Digest verification of the published file still covers all 1,488,915,288 bytes, including the unused towers. [Tensor reader](https://github.com/ex1tium/rembed/blob/f401a5543612a4b74ff6baac4cac1695b2cb6876/internal/safetensors/reader.go)

## Parity against sentence-transformers

I ran the same twelve documents and ten queries as the goinfer test, one text per call, default non-int8 inference, full context. I compared each result to the saved sentence-transformers f32 reference and independently generated the reference token IDs from the same pinned tokenizer. The 512-dimensional check takes the first 512 coordinates of each vector and uses cosine, equivalent to truncation and renormalisation. The gate is **≥0.999 for every text**. [goinfer reference and fixtures](./goinfer-embeddinggemma2.md)

| Inputs | Count | Lowest cosine, 768 | Lowest cosine, 512 | Exact token IDs |
|---|---:|---:|---:|---:|
| Corpus, code, German, Japanese, short and long documents | 11 | 0.9999999999970 | 0.9999999999969 | 11/11 |
| Queries | 10 | 0.9999999999992 | 0.9999999999992 | 10/10 |
| Literal `<eos>` document | 1 | **0.9520045990** | **0.9522391439** | **0/1** |

The reference recognises the embedded `<eos>` as token ID 1 and returns 22 tokens. rembed treats those characters as ordinary text and returns 23. It still adds the proper terminal EOS. To isolate the cause, I fed the reference IDs for just this document into the same fork's internal model. Cosine rose to **0.9999999999996**. The difference is in tokenisation. Treating literal markers as text may suit pbrain's handling of untrusted input. To compare that policy fairly, apply the same preparation to both implementations, fingerprint it, and retain the raw parity regression. goinfer matched the raw reference on this case. [Gemma tokenizer](https://github.com/ex1tium/rembed/blob/f401a5543612a4b74ff6baac4cac1695b2cb6876/tokenizer/gemma/gemma.go)

## Four-thread measurements

Same Intel Core Ultra 7 255H, Linux amd64, Go 1.27.3, warm filesystem cache, `GOMAXPROCS=4` and `WithWorkers(4)`. Each startup was a fresh process, with the rig's first query and five repeats of its warm query. The unmodified library received one input per call. The laptop was shared during this run, so small timing differences between studies are inconclusive.

| Measurement | rembed at the fork pin |
|---|---:|
| Load a local checkpoint, three runs | 1046–1091 ms |
| First query after load | 36–51 ms |
| Load and first query, before any digest check | **1098–1126 ms** |
| Warm query, 15 calls | 34–49 ms |
| Peak RSS, local load and short queries (without Artifact bytes) | 1.62 GB |
| 30 corpus windows, 15,211 tokens including prompts | 29.19 s, **521 tokens/s**, 1.62 GB peak RSS |
| Full 50-section retrieval corpus, up to 3,792 tokens per input | 1.75 GB peak RSS |

A separate probe read the official 1.49 GB safetensors file into an owned byte slice and checked its pinned SHA-256 in **1185–1603 ms** across three fresh processes. That timing covers the weights alone. There is no successful end-to-end `Verify → Open(Artifact) → query` measurement for rembed because its loader accepts paths only. A diagnostic split of one load measured 435 ms parsing the tokenizer and 780 ms loading weights on a shared machine. The 2,294-token parity document took 82.7 s and the longest evaluation section (3,792 tokens) took 159.8 s. These are full-document measurements. pbrain would index shorter passages. The encoder computes attention scores across each full row before applying the sliding mask, which makes long-input results especially important to measure. [Attention loop](https://github.com/ex1tium/rembed/blob/f401a5543612a4b74ff6baac4cac1695b2cb6876/internal/model/gemma2emb.go#L374)

## The 20-question retrieval check

I encoded all 50 original spec sections and 20 questions. Each question ranked the sections by cosine. The acceptable section was first for **9/20**, and among the top three for **12/20**. The earlier runs yielded 9/20 and 12/20 for goinfer with the same model, and 13/20 and 17/20 for bge-small/hugot. These are answerable English questions. The architecture's release fixture also needs unanswerable questions, lexical-first hybrid output and a held-out cutoff test. [goinfer results](./goinfer-embeddinggemma2.md) · [bge-small/hugot results](./spike-bge-small-hugot.md)

## Builds and integration

I read the loader, text model, tokenizer, tensor reader, hub and process-side effects before running the fork. Runtime probes used a scratch copy of the checkpoint, an explicit local directory and a clean child environment. The test program links the library, takes a role and a JSON list of texts, and writes vectors, token IDs and timings to an explicit output file. The comparison executable keeps the same standard-library argument, JSON, hashing and filesystem work. Both use `CGO_ENABLED=0 go build -trimpath -ldflags='-s -w'`. Sizes below are decimal MB of stripped, uncompressed binaries.

| Target | Probe | Matched baseline | Added |
|---|---:|---:|---:|
| Linux amd64 | 7.63 | 2.81 | 4.82 |
| Linux arm64 | 7.01 | 2.69 | 4.33 |
| macOS amd64 | 7.81 | 2.87 | 4.94 |
| macOS arm64 | 7.18 | 2.67 | 4.52 |

Both Linux binaries are static. The macOS binaries use the system libraries and frameworks normally linked by Go networking code (libSystem, libresolv, CoreFoundation, Security). All four link without a separate inference library. Only Linux amd64 was executed. The optional Python C API is outside the Go import used here.

**Verified model bytes.** The released API is `Load(ref string, opts ...Option)`. Its local branch opens configuration and tokenizer files by pathname and maps the weights by pathname. A prior hash of an owned byte slice does not bind this loader to those bytes. The v2 path also cannot use the Qwen-specific disk pack. To meet pbrain's contract, a fork would need byte-backed metadata/tokenizer parsers and a safetensors reader over the verified slice. An adapter must never reopen or stage mutable copies of the model files. [Loader](https://github.com/ex1tium/rembed/blob/f401a5543612a4b74ff6baac4cac1695b2cb6876/rembed.go#L139) · [Mapped reader](https://github.com/ex1tium/rembed/blob/f401a5543612a4b74ff6baac4cac1695b2cb6876/internal/safetensors/reader.go#L25)

**Environment and output.** The same loader also accepts Hub identifiers. That branch can read `REMBED_CACHE`, Hugging Face token variables and a token file, perform HTTP requests and write a cache. An integration should expose only the verified-bytes constructor and leave downloads to pbrain. I found no backend-specific environment initialiser or direct print in the embedded local path. [Hub and credential handling](https://github.com/ex1tium/rembed/blob/f401a5543612a4b74ff6baac4cac1695b2cb6876/internal/hub/hub.go)

The local success path emitted zero bytes on stdout and stderr across the measurement runs. A separate Linux trace with fixture-only cache and credential settings and a malformed GoMLX setting saw no network connection or access to those cache/credential locations. The probe wrote its requested result file. This observation covers local load and inference, not the optional Hub or CLI.

## Adoption decision

Use goinfer to evaluate EmbeddingGemma 2's quality now. It already supplies reference-correct vectors for the fixture. Keep the accepted bge-small/hugot adapter until an alternative passes the full parity, verified-startup and retrieval gates on the target machines. If the owner funds a runtime, the goinfer prototype provides a better starting point: a measured path to cheaper weight loading and fewer tokenizer differences to resolve. Its full startup estimate still depends on a fast tokenizer and a pinned text-only pack. Keep rembed's token-table implementation and short-query speed as engineering references. The current evidence supports preserving the Embedder interface and the existing gates. [goinfer architecture review](./goinfer-architecture-review.md) · [Architecture §8.7](../../../architecture.md)
