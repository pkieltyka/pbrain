# Spike: bge-small-en-v1.5 on hugot's pure-Go backend

Run 2026-11-21 by @lead-dev-2, for architecture 8.7 (sha256 d7a34fb3) and the adapter contract in `components/semantic.md` (b029d3e7).

Setup:
- the machine and method of [cpu-measurements.md](./cpu-measurements.md): an Intel Core Ultra 7 255H, Linux x86_64 with AVX2, Go 1.27.3, 4 threads (`GOMAXPROCS=4`) unless stated, warm file cache;
- single runs.

Before anything was built or run, the modules were downloaded into a scratch folder and their source was read, by a sub-agent's file-by-file audit and my own spot checks. Nothing was installed outside the scratch folder. "Inference:" marks my reading.

## Bottom line

**Accept bge-small-en-v1.5 on hugot v0.8.6's Go backend as the interim runtime, with four conditions.** It passes rules 1 to 3 of architecture 8.7. Rule 4, the release gates on the fixture, is for the semantic step itself.

- **Rule 1, pure Go: passes.**
  - Every compiled package has no cgo, also with `CGO_ENABLED=1`.
  - No shared library.
  - It builds for linux and darwin on amd64 and arm64.
- **Rule 2, reference vectors: passes.**
  - On the same 22 texts as the goinfer check, the token ids are identical to sentence-transformers' for every text.
  - The worst 1 − cosine is 4.6e-13 without SIMD and 2.8e-10 with it.
- **Rule 3, verify, load and one query within 600 ms: passes.**
  - With `GOEXPERIMENT=simd`: 440 to 485 ms (verify 101 to 132 ms, load 269 to 306 ms, first query 65 to 69 ms).
  - Without it: 525 ms.
- **The contract's other points pass:**
  - the model is built from the verified bytes alone;
  - a word at the start of a passage still changes the vector under the longest prefix;
  - literal special-token text is neutralised;
  - vectors are the same in any batch.

The conditions:

1. **Build releases with `GOEXPERIMENT=simd`.**
   - Without it, indexing runs at about 410 tokens/s against 3,700 to 4,100 with it, so a first index of 5,000 items takes about 95 minutes instead of about 10.
   - The flag is an experiment in Go 1.27. Its arm64 build compiles, but I could not run it, so measure the arm64 builds on a Mac before release.
2. **Ship the adapter shim of section 3,** or upstream a fix. hugot reads the ONNX file with `os.ReadFile` unless the model path starts with `s3:`. The shim uses that path through an in-memory file system. The one-line upstream fix is better.
3. **The adapter does its own tokenising, truncation, special-token neutralisation and CLS pooling** (section 3). hugot's tokenizer ignores the length limit, parses `[SEP]` in text as the real token, and offers only mean pooling, while bge needs the CLS token.
4. **Pin the versions and digests of section 8,** and keep a test that stdout stays empty: one hugot fallback path prints to stdout.

The binary grows by about 18 MB on arm64 and 24 MB on amd64.

## 1. Rule 1: pure Go, builds, size

- `CGO_ENABLED=0 go list -deps` lists no package with a cgo file, also with `CGO_ENABLED=1`.
- **Assembly:**
  - The one `.s` file linked is `compute/internal/gobackend/dot/matmul/epilogue_amd64.s` (AVX2/AVX-512 bias-add loops), and it is called only under `GOEXPERIMENT=simd`.
  - The SIMD paths use Go's `simd/archsimd` (experiment), with plain-Go fallbacks.
  - `x/sys/unix` has its usual syscall stub.
- **Builds,** `CGO_ENABLED=0 go build -trimpath -ldflags='-s -w'` of a program that loads the model and embeds text:

| Target | With `GOEXPERIMENT=simd` | Without | A program without hugot |
|---|---|---|---|
| linux/amd64 | 26.3 MB | 20.4 MB | 2.6 MB |
| linux/arm64 | 20.6 MB | 17.7 MB | |
| darwin/amd64 | 26.7 MB | 20.8 MB | |
| darwin/arm64 | 21.1 MB | 18.2 MB | 2.5 MB |

hugot adds about 18 to 24 MB. Inference: much of it is pipelines pbrain never uses (image, audio, video, generative), linked through hugot's root package. Importing only `hugot/backends` and `hugot/pipelines` might trim it; not tried.

Only linux/amd64 was run. The other three were built, not executed.

## 2. Rule 2: reference vectors

Reference: sentence-transformers 6.3.0 (transformers 5.23.0, torch 2.15.0, CPU) loading `BAAI/bge-small-en-v1.5` at the revision pinned below: CLS pooling, normalised, `max_seq_length` 512. Queries carry the model's retrieval instruction, `Represent this sentence for searching relevant passages: `; documents carry none.

| Texts | Token ids against the reference | Worst 1 − cosine, no SIMD | With SIMD |
|---|---|---|---|
| 12 documents (6 engineering windows of 398 to 714 words, truncated to 512 tokens; Go; German; Japanese; "Redis"; a 2,294-token text truncated; a text with a literal `<eos>`) | identical, all 12 | 4.6e-13 | 2.8e-10 |
| 10 queries | identical, all 10 | 3.5e-13 | below 2.8e-10 |

The largest component difference was about 1e-7. With SIMD the vectors move by 2.8e-10 from the non-SIMD ones. The same text in batches of 1 and 8 differs by at most 9e-14.

## 3. Building from the Artifact's bytes alone: the shim

hugot reads files through a `fileutil.FileSystem` bound to the session (`options.WithFileSystem`, `hugot.go:90`), so an in-memory file system can serve `config.json` and `tokenizer.json`. The ONNX file is the exception. `createGoMLXModelBackend` reads it with `parser.ParseFile`, which uses `os.ReadFile` (`backends/model_gomlx.go:162`, `onnx-gomlx/internal/onnxgomlx/onnxgomlx.go:160`), unless `model.OnnxReader` is set. And `CreateModelBackend` sets that reader only for a model path starting with `s3:` (`backends/pipeline.go:427-433`).

**The shim used here:**
- an in-memory `FileSystem` holding the three verified files under `s3://pbrain-mem/model/`, with `OpenFile`, `FileExists`, `FileStats` and `Walk`;
- every write, copy and delete refused;
- `ModelPath: "s3://pbrain-mem/model"`;
- hugot has no S3 client: the `s3:` prefix only routes the read through the injected file system.

The file system's call log shows only `config.json`, `tokenizer.json` (twice) and `model.onnx` read, plus existence checks for two optional files.

An `strace` of a whole run (verify, load, ten queries) shows each model file opened exactly once, by the verification step's own read. Otherwise only `/proc/self/{status,mountinfo,cgroup}`, a transparent-huge-page setting under `/sys` and the executable itself were opened; the last is most likely the Go runtime. Nothing opened the model folder after verification. A model parsed from a reader has no base folder, so ONNX external data, which onnx-gomlx would open with `os` and no containment check, cannot be read. bge's `model.onnx` has none.

**Better than the shim:** a small upstream change, using `fileutil.OpenFile` whenever the context's file system is not the default OS one (`fileutil.IsDefaultOSFileSystem`), would remove the `s3:` trick. It is worth offering to knights-analytics/hugot; until then the shim is ten lines and covered by the adapter's tests.

**The rest of the adapter** (all checked in the probe):
- **Tokens.** The adapter tokenises with hugot's tokenizer (pure-Go `hftokenizer` from `tokenizer.json`: BertNormalizer, lower-casing, WordPiece) with special tokens off.
  - It adds `[CLS]` and `[SEP]` itself.
  - It builds the batch with the exported `backends.NewBatch`, `CreateInputTensors` and `RunSessionOnBatch`.
  - It takes token 0 of `last_hidden_state` and L2-normalises it.
  - hugot's own pipeline would mean-pool, which is wrong for bge, and never truncates: `MaxLen` is set but the tokenizer never reads it.
- **Cut rule** (semantic.md, adapter contract). The limit is 510 tokens between `[CLS]` and `[SEP]`. A document is split at its first blank line into prefix and passage. If they exceed the limit, the prefix is cut to fit, but never below 255, so the passage always keeps at least half; then the passage is cut from its end. A query is cut from its end. WordPiece never makes a token across white space, so the split gives the same ids as tokenising the whole text whenever it fits.
- **Special tokens.** hugot's tokenizer matches `[PAD]`, `[UNK]`, `[CLS]`, `[SEP]` and `[MASK]` anywhere in the raw text (`hftokenizer.go:362-406`). So does the Python reference, so this is parity, but the contract forbids it.
  - The adapter rewrites each such string with a space after its first character (`[SEP]` becomes `[ SEP]`), which BertPreTokenizer splits into the same `[`, `sep`, `]` pieces a tokenizer with no special matching would give.
  - Checked: `a [SEP] b [CLS] c` encodes as 102 and 101 raw. The adapter gives `[`, `sep` (19802), `]` and `[`, `cl`, `##s`, `]`, and `[ SEP]` gives the same ids.
- **Prompts:** the query instruction above, none for documents. These are the adapter's preparation version.

## 4. Rule 3 and speed

**Verify, load and one query**, one process, `GOMAXPROCS=4`:
- verify = read and SHA-256 the three files, 134 MB;
- load = session, ONNX parse, graph build;
- query = tokenise, run, pool.

| Build | Verify | Load | First query | Total | Peak RSS |
|---|---|---|---|---|---|
| `GOEXPERIMENT=simd`, three runs | 101 to 132 ms | 269 to 306 ms | 65 to 69 ms | 440 to 485 ms | 0.60 GB |
| no SIMD | 121 ms | 305 ms | 99 ms | 525 ms | 0.60 GB |

- **Warm queries** (the same process, ten in a row): 23 to 27 ms with SIMD, 60 to 73 ms without.
- **Peak RSS** includes the in-memory copy of the files (134 MB), the parsed graph and the weights.

**Indexing:** 30 windows of the corpus (mean 299 words, 471 tokens after truncation, 14,142 tokens in all):

| Build | Threads | Batch | Tokens/s | Per window | Peak RSS |
|---|---|---|---|---|---|
| `GOEXPERIMENT=simd` | 4 | 1 | ~3,700 | ~127 ms | 0.82 GB |
| `GOEXPERIMENT=simd` | 4 | 8 | ~4,100 | ~115 ms | 0.88 GB |
| `GOEXPERIMENT=simd` | 16 | 8 | ~5,550 | ~85 ms | 0.89 GB |
| no SIMD | 4 | 1 | ~410 (12 documents) | ~1,250 ms per 512 tokens | 0.71 GB |

- **5,000 items** (inference): at about 470 tokens an item, about 2.4 M tokens, so a first index takes about 10 minutes at 4 threads with SIMD (about 7 at 16), and about 95 without SIMD.
- **The earlier figures:** the 0.22 s load and 1.9 GB peak in architecture 8.7 came from my first hugot run, at 16 threads, with whole texts untruncated (hugot does not truncate). With the adapter's truncation the peak is under 0.9 GB at batches of 8.
- **Threads:** the Go backend sizes its worker pool from `GOMAXPROCS` only (`compute/internal/gobackend/scheduler/scheduler.go:58-62`); hugot passes it no option. pbrain can cap threads with `runtime.GOMAXPROCS` before loading. The workers are never stopped, which does not matter for one command in one process.

## 5. The longest prefix (contract test)

- **The prefix**, at its permitted maximum:
  - a 200-code-point title in Chinese (one token per character after BertNormalizer's CJK spacing);
  - a 200-code-point section line of `a-a-a-…` (one token per character);
  - 400 tokens in all, cut by the adapter to 255.
- **The passages:** a 300-word window, differing only in the first word, then only in the last word. Both texts reached the full 512 tokens.

| Change | Cosine between the two vectors |
|---|---|
| first word of the passage (`Rollback` against `Deployment`) | 0.9863: the start of the passage is in the vector |
| last word of the passage (`zebra` against `quantum`) | 1.0000000: the cut tail is not, as the contract states |
| the two single words alone, with no prefix | 0.634 |

The first-word change is small next to two short texts, because the 255 prefix tokens and the rest of the passage are shared, but it is a real difference.

## 6. The 20-question check

The same 50 sections of the functional spec and 20 reworded questions as in cpu-measurements.md. Sections were truncated at 512 tokens from their end, as before, and queries carry the retrieval instruction, as before.

| Model and runtime | Right section first | In top 3 |
|---|---|---|
| bge-small-en-v1.5, hugot pure Go (this spike) | 13 | 17 |
| bge-small-en-v1.5 int8, ONNX Runtime (earlier) | 13 | 17 |
| EmbeddingGemma v1 Q8_0, llama.cpp (earlier) | 15 | 19 |
| EmbeddingGemma 2, goinfer (earlier) | 9 | 12 |

The same result as the earlier bge run, as it should be: the vectors are the reference's.

## 7. Safety review

Source read at the versions of section 8 (a sub-agent's audit with file and line citations, spot-checked by grep).

- **Network and processes.**
  - `net/http` is linked through go-huggingface's `hub` downloader. hugot's tokenizer package imports `hub`, so the `NODOWNLOAD` build tag alone does not remove it.
  - `os/exec` is linked through hugot's video decoder (`ffprobe` and `ffmpeg`).
  - Neither is reached from `NewGoSession`, `NewPipeline` or a run. A missing file is an error, never a download.
  - The only start-up side effect in `hub` is a random session id for its User-Agent.
- **No file is written** on this path. klog defaults to standard error and writes no log file.
- **Output on stdout and stderr.**
  - hugot prints with `fmt.Printf` to **stdout** when a dynamic-shape build fails and it retries with buckets (`backends/model_gomlx.go:119`). That did not happen with bge, but it would break pbrain's stdout contract, so the adapter's test asserts that nothing is printed.
  - klog prints some runtime errors to stderr.
- **Environment.**
  - `GOMLX_GO_*` variables tune or disable SIMD kernels, and an invalid value panics or exits.
  - `GOMLX_FUSION` and `GOMLX_OP_PROFILE` (which prints a table to stdout) are also read.
  - `GOMLX_BACKEND` is ignored by hugot.
  
  Inference: pbrain can't stop a user's environment reaching these. A bad value fails the command, and `GOMLX_OP_PROFILE` would print to stdout. This is worth a line in the troubleshooting docs, or in the adapter's start-up: clear these variables for the process before loading.
- **Hostile model files.** onnx-gomlx would follow `../` in an ONNX external-data path. pbrain only ever loads a file whose SHA-256 is pinned, and the shim gives no base folder, so this is not reachable.
- **Licences:**
  - hugot: Apache-2.0;
  - the gomlx modules: Apache-2.0;
  - bge-small-en-v1.5: MIT, ungated.

## 8. What to pin

**Go modules** (go.sum, 2026-11-21; the latest releases):

| Module | Version | h1 |
|---|---|---|
| github.com/knights-analytics/hugot | v0.8.6 | `8kK6PlJeZCGT5Vl7Lu/Zzru90EXzX6NAdT/n1U69Rd8=` |
| github.com/gomlx/compute | v0.1.28 | `3o0pCmmg3QzZMMS5PR6nUnyRzEMPXI0uGrzcIP+qpPU=` |
| github.com/gomlx/compute-onnx | v0.1.28 | `hoySn1M2KlBrmyfH7Hqmll5HE0dq+n6Saai0X0ODmcY=` |
| github.com/gomlx/gomlx | v0.28.34 | `oUsFSClHMNVnI7hfFdHPxeqy7tD5ggtD64wdjHrB7ds=` |
| github.com/gomlx/onnx-gomlx | v0.5.31 | `ulTu+9GOp/1EEP5MF9S+Hpp6abCVz8C2ksBj5eZ9ra0=` |
| github.com/gomlx/go-huggingface | v0.4.32 | `bw/nY5FAgVWTVX7ZOxsK6FhGn/hz2CdwKax00heFpBw=` |

**Model files**, from `BAAI/bge-small-en-v1.5` at revision `5c38ec7c405ec4b44b94cc5a9bb96e735b38267a`, at `https://huggingface.co/BAAI/bge-small-en-v1.5/resolve/<revision>/<path>`. These three are the table row's `Files`:

| File (path in the repository) | Size | SHA-256 |
|---|---|---|
| `onnx/model.onnx` | 133,093,490 | `828e1496d7fabb79cfa4dcd84fa38625c0d3d21da474a00f08db0f559940cf35` |
| `tokenizer.json` | 711,396 | `d241a60d5e8f04cc1b2b3e9ef7a4921b27bf526d9f6050ab90f9267a1f9e5c66` |
| `config.json` | 743 | `094f8e891b932f2000c92cfc663bac4c62069f5d8af5b5278c4306aef3084750` |

`config.json` is optional for hugot, but it sets the 512-position limit hugot reports, so it is kept. `Dim` is 384.

**Preparation version** (part of the fingerprint):
- the query instruction above, and none for documents;
- CLS pooling and L2 normalisation;
- the 510-token cut rule, with the prefix kept to at least 255 tokens only when the passage is shorter;
- special-token neutralisation.

## 9. Risks and open points

- **`GOEXPERIMENT=simd`** is a Go experiment. A toolchain upgrade can break or change it, and arm64 performance under it is unmeasured. Without it, rule 3 still passes (525 ms), but a first index takes about 95 minutes.
- **The `s3:` shim** relies on an internal branch of hugot. A hugot release could change it, so it needs a pinned version and a test; an upstream fix is the durable answer.
- **Binary size:** 18 to 24 MB on top of pbrain.
- **hugot pace:** v0.8.x, with frequent releases and a large surface (ORT, XLA, generative, video). pbrain uses a small corner of it; pin and review on each upgrade.
- **The model:** bge-small-en-v1.5 is English, with 512 tokens and 384 dimensions. It placed 13 of 20 first on our check, below EmbeddingGemma v1 (15), above EmbeddingGemma 2 (9). The fixture decides whether it is good enough, and calibrates its cut-off.
- **Not measured:**
  - a cold-cache load;
  - the arm64 and darwin builds running;
  - pbrain's own end-to-end `search --semantic` time, which the semantic step measures.

## Sources

- **Code:** hugot v0.8.6 and the gomlx modules above, from proxy.golang.org: `fileutil/file.go`, `options/options.go`, `backends/{model,model_gomlx,pipeline,tokenizer,tokenizer_go}.go`, `pipelines/{featureExtraction,rawFeatures}.go`, onnx-gomlx `internal/onnxgomlx/onnxgomlx.go` and `filesreader`, go-huggingface `tokenizers/hftokenizer`, compute `internal/gobackend/{scheduler,simdarch,dot/matmul}`.
- **Model:** `BAAI/bge-small-en-v1.5` (Hugging Face API, 2026-11-21): `modules.json`, `1_Pooling/config.json` (`pooling_mode_cls_token: true`), `sentence_bert_config.json` (`max_seq_length: 512`), `tokenizer.json`.
- **Local runs, 2026-11-21:**
  - a 250-line Go probe with the in-memory file system, the adapter's tokenising, cut rule, neutralisation and CLS pooling, and timing;
  - a sentence-transformers reference script;
  - the earlier `eval.py` questions;
  - `strace`.
  
  All of it is in this session's scratch folder, not committed.
