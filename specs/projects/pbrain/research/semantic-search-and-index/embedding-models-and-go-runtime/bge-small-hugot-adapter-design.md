# bge-small-en-v1.5 on hugot: the adapter design, kept for the fallback

This is the design for running bge-small-en-v1.5 on hugot's pure-Go backend, as it stood in `components/semantic.md` (sha256 `8af1b16e…`) and `architecture.md` (sha256 `380a85e5…`) when both reviewers approved them on 2026-11-21. The owner then chose aikit's encoder for this model ([goinfer-bge-small-check.md](./goinfer-bge-small-check.md)), and hugot became the fallback. Nothing here is built or linked. The measurements behind it are in [spike-bge-small-hugot.md](./spike-bge-small-hugot.md).

The text of each section below is copied unchanged from those two files; only the headings are new. Where it says "the spike" it means the hugot spike, and its section numbers are those of the architecture at that hash.

## What going back would change

- One adapter package (`internal/embed/gohugot` in place of `internal/embed/bge`) and one row of the model table. The row names the ONNX weights, so its fingerprint differs and every index is built again.
- `internal/envguard` returns, imported by `cmd/pbrain` and by every program that links the backend.
- Release builds set `GOEXPERIMENT=simd`.
- hugot and five GoMLX modules return to `go.mod`, with 18 to 24 MB of binary.
- Nothing above the `embed` seam changes. The preparation (`bge-1`) is the same in both adapters.

## The conditions in the architecture (section 8.7)

**bge-small on hugot: the spike's conditions** are part of the design (`components/semantic.md`, "The bge-small adapter"; numbers in `spike-bge-small-hugot.md`, one Intel laptop, one run each):

- **Release builds set `GOEXPERIMENT=simd`.** Without it the 600 ms rule still holds (525 ms) but a first index takes about 95 minutes instead of 10. It is an experiment of the Go toolchain, and no arm64 build has been run: the arm64 builds are measured on a Mac before a release.
- **The model is built from memory** through an in-memory file system that hugot reaches only for a model path starting with `s3:`. No S3 client exists in it; the prefix chooses the code path. It rests on one internal branch of hugot, so the module is pinned and a test guards it; a small upstream change is the durable answer.
- **The adapter does the model's own preparation**, because hugot's pipeline does not: tokenising, the cut to 510 tokens that leaves the passage at least 255, making literal special-token text harmless, and pooling on the first token.
- **Modules and model files are pinned** by version and digest (section 15 and the component design).
- **The backend's own environment variables are removed before it starts.** It reads `GOMLX_…` variables while its packages are initialised: an invalid value stops any command, and one of them prints on standard output. A package that imports only `syscall` removes them first, which Go's initialisation order guarantees today and a test on the built binary checks on every target (section 14).

## The modules (architecture section 15)

| Module | Version | Used for |
|---|---|---|
| `github.com/knights-analytics/hugot` | v0.8.6 | The runtime for `bge-small-en-v1.5`, pure-Go backend, behind `embed.Registry` (section 8.7). |
| `github.com/gomlx/gomlx`, `gomlx/compute`, `gomlx/compute-onnx`, `gomlx/onnx-gomlx`, `gomlx/go-huggingface` | v0.28.34, v0.1.28, v0.1.28, v0.5.31, v0.4.32 | What hugot's Go backend runs on: the tensor backend, the ONNX reader and the tokenizer. Pinned with hugot; their checksums are in the spike report. |

Release builds also set `GOEXPERIMENT=simd` for the embedding runtime. The runtime adds 18 to 24 MB to the binary, and it links packages for network and subprocess use that pbrain never reaches; the spike read that code and the adapter's tests hold it to no network call and no file opened.

## The adapter (components/semantic.md, "The bge-small adapter")

Package `internal/embed/gohugot` is the only package that imports hugot. Every fact below was checked in the spike; names of hugot's functions are those of v0.8.6.

```go
package gohugot

// Entry is the release row for bge-small-en-v1.5 and its opener.
func Entry() embed.Entry
```

**The model row.**

| Field | Value |
|---|---|
| `Name` | `bge-small-en-v1.5` |
| `Runtime` | `hugot-go` |
| `Dim` | 384 |
| `Cutoff` | set by the calibration tool before the row ships; the row is not in a release until both gates pass |
| Preparation version | `bge-1`: the query instruction, CLS pooling, L2 normalisation, the 510-token cut rule and the special-token rule below |

Files, from `https://huggingface.co/BAAI/bge-small-en-v1.5/resolve/5c38ec7c405ec4b44b94cc5a9bb96e735b38267a/<path>`; `File.Name` is the local name:

| `File.Name` | Path in the repository | Size | SHA-256 |
|---|---|---|---|
| `model.onnx` | `onnx/model.onnx` | 133,093,490 | `828e1496d7fabb79cfa4dcd84fa38625c0d3d21da474a00f08db0f559940cf35` |
| `tokenizer.json` | `tokenizer.json` | 711,396 | `d241a60d5e8f04cc1b2b3e9ef7a4921b27bf526d9f6050ab90f9267a1f9e5c66` |
| `config.json` | `config.json` | 743 | `094f8e891b932f2000c92cfc663bac4c62069f5d8af5b5278c4306aef3084750` |

**Opening from memory.** hugot reads `config.json` and `tokenizer.json` through a file system given to the session (`options.WithFileSystem`), but it reads the ONNX file from disk unless the model path starts with `s3:`, in which case it uses the same injected file system. hugot has no S3 client, so the prefix only chooses the code path. The opener therefore:

1. Builds a read-only in-memory file system (hugot's `fileutil.FileSystem`: open, exists, stat and walk; every write, copy and delete refused) that holds the three byte slices of the `Artifact` under `s3://pbrain-mem/model/`.
2. Creates a Go-backend session (`hugot.NewGoSession`) with that file system and loads the model from the path `s3://pbrain-mem/model`. No file is opened after `Verify`: a model parsed from a reader has no base folder, so the external-data feature of ONNX cannot reach the disk either.
3. Loads hugot's tokenizer from the same file system with special tokens off.

This relies on one internal branch of hugot, so the module is pinned and `TestHugotBuildsFromBytes` guards it. The durable answer is a small change upstream (read through the injected file system whenever it is not the operating system's); it should be offered, and the shim removed when it is released.

**The backend's environment variables.** The GoMLX backend that hugot uses reads variables named `GOMLX_…` by itself, and it does so while its packages are initialised, before `main` runs: an invalid value stops the process, and `GOMLX_OP_PROFILE` makes it print on standard output. Because the backend is linked into the binary, this would hit every command, `pbrain --version` included, and nothing in `cli`, `main` or the adapter runs early enough to prevent it. pbrain reads its own environment only through `Env` and gives these variables no meaning, so they are removed:

- Package `internal/envguard` imports only `syscall`. A package-level initialiser removes every variable whose name starts with `GOMLX_` from the process. `cmd/pbrain` imports it for that effect alone, and so does every other program in the repository that links the backend: the calibration tool and any helper that opens the adapter.
- It runs first because of Go's initialisation order: among the packages whose dependencies are ready, the one with the smallest import path is initialised next. `envguard` is ready as soon as `syscall` is; every backend package that reads the environment needs `os`, which is ready later.
- Tried on 2026-11-21 against `github.com/gomlx/compute` v0.1.28 with Go 1.27.3 on linux/amd64, with and without `GOEXPERIMENT=simd`: with five invalid `GOMLX_` values and `GOMLX_OP_PROFILE=1` inherited, the binary without the guard stops in initialisation, and the binary with it starts and prints only its own output.
- The order is a property of the toolchain and of the backend's imports, so it is not taken on trust: `TestReleaseBinaryIgnoresBackendEnv` runs the built binary on every target. If a toolchain or module upgrade makes it fail, the fallback is a reviewed, pinned patch of the backend's read sites through a `replace` directive. Asking upstream to make these reads optional is worth doing either way.

**Stray output.** hugot prints with `fmt.Printf` when a dynamic-shape build fails and it retries another way, and its logger can write to standard error. Neither happened with this model, but a test of the normal path cannot show that a failure path stays silent, and one raw line would break the output contract of spec 6.3. So the guarantee is made where it cannot be missed: `cmd/pbrain` takes the process's real standard output and standard error for `Env.Stdout` and `Env.Stderr`, then points the variables `os.Stdout` and `os.Stderr` at the null device, before any command runs. `cli` writes only through `Env`, so its output is unchanged, and anything a linked library prints through those variables is discarded, whatever path it takes. A crash of the Go runtime still reaches the real standard error, because the runtime writes to the descriptor itself.

**`Embed`.** The adapter does its own tokenising, cutting and pooling, because hugot's pipeline never cuts an input, reads literal special-token text as the real token, and only mean-pools.

1. **Special tokens.** In each text, every literal `[PAD]`, `[UNK]`, `[CLS]`, `[SEP]` and `[MASK]` gets a space after its first character (`[SEP]` becomes `[ SEP]`). The tokenizer then gives the ordinary pieces, the same as for the text with no special meaning.
2. **A query** gets the instruction `Represent this sentence for searching relevant passages: ` in front, is tokenised, and is cut from its end to 510 tokens.
3. **A document** is split at its first blank line into prefix and passage; with no blank line the whole text is the passage. Each part is tokenised on its own, which gives the same tokens as the whole text, since no token crosses white space. If together they pass 510 tokens, the prefix is cut from its end to `max(255, 510 - passage tokens)`, then the passage is cut from its end to what remains. The passage therefore always has at least 255 tokens of room.
4. The tokens are wrapped as `[CLS]` … `[SEP]`, the texts of one call are padded into one batch (`backends.NewBatch`, `CreateInputTensors`, `RunSessionOnBatch`), and for each text the vector is token 0 of `last_hidden_state`, L2-normalised.
5. The context is checked before each call; a call in progress is not interrupted.

`Close` destroys the session. The worker pool takes its size from the number of processors Go is allowed; the adapter does not change it.

**Measured in the spike** (one Intel laptop, 4 threads, one run each): token ids equal to the reference on 22 texts and vectors within 2.8e-10 of it; verify, load and first query 440 to 485 ms with `GOEXPERIMENT=simd` and 525 ms without; a search peaks at 0.60 GB, the 134 MB of model bytes included; indexing at batches of eight runs at about 4,100 tokens a second and peaks under 0.9 GB, so a first index of 5,000 items takes about 10 minutes, and about 95 without the experiment. Release builds therefore set `GOEXPERIMENT=simd`. Not measured: any arm64 or macOS run, and a cold file cache.

## The tests

| Test | What it shows |
|---|---|
| `TestHugotParity` | With the model files present: for the 22 reference texts kept in `testdata` (documents and queries, code, German, Japanese, over-long texts), token ids equal the stored reference ids and each vector has cosine similarity of at least 0.999 with the stored reference vector; a batch of one and a batch of eight give the same vectors. |
| `TestHugotBuildsFromBytes` | The model folder is deleted after `Verify` and before `Open`: the embedder opens and embeds. The in-memory file system refuses every write. Guards the `s3:` path against a hugot upgrade. |
| `TestHugotCutRule` | A 600-token query is cut to 510 from its end. A 400-token prefix with a 300-token passage: the prefix is cut to 255 and the passage to 255. A 400-token prefix with a 50-token passage: the prefix keeps 400 and nothing is cut. Changing the passage's first word changes the vector; changing a word in its cut tail does not. |
| `TestHugotSpecialTokens` | `a [SEP] b [CLS] c` gives the ids of the plain pieces, never ids 101 or 102 inside the text; a text with and without the inserted space gives the same ids. |
| `TestHugotQuiet` | In a child process whose standard output is a pipe the test reads: opening the model and embedding print nothing there. |
| `TestReleaseBinaryIgnoresBackendEnv` | The built release binary, started as a fresh process with `GOMLX_GO_SIMD_AVX2=notabool`, `GOMLX_GO_SIMD_AVX512=zzz`, `GOMLX_GO_AVX2_ASM=bad`, `GOMLX_GO_FUSION=maybe`, `GOMLX_FUSION=maybe`, `GOMLX_GO_AVX2_KC=abc`, `GOMLX_TENSOR_SUMMARY_SAMPLE_SIZE=x` and `GOMLX_OP_PROFILE=1` inherited, one case per fatal read site and one with all together: `--version`, a lexical `search`, `search --semantic` and `index` each exit as they do without those variables, and the bytes read from the process's real standard output pipe are exactly the command's output. The `index` case embeds several batches and the search case runs after it, so output that a profiler would only print after repeated calls or at exit is caught too. The calibration tool is run the same way. Run on every release target, with and without `GOEXPERIMENT=simd`. |

The architecture's test row for the release binary (section 14):

| Release binary | The built binary of every target, started as a fresh process with invalid `GOMLX_…` values and `GOMLX_OP_PROFILE` inherited: `--version`, a lexical search, a search with `--semantic` and `index` exit as they do without them, and the process's real standard output holds only the command's output. With and without `GOEXPERIMENT=simd`. A second child process shows that a print through the `os.Stdout` or `os.Stderr` variable reaches neither real stream after start-up, while `Env`'s writers do. |
