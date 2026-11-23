# pbrain, next release

Rename this file to the release tag, such as `v0.3.0.md`, when the version is chosen.
The release workflow reads `docs/release-notes/<tag>.md`. Add the other changes made
since v0.2.0 before tagging.

## Install

The standard Linux amd64 archive now includes both GPU backends: CUDA for NVIDIA GPUs
and Vulkan for discrete AMD GPUs. The standard Linux arm64 archive includes CUDA alone,
because the Vulkan backend supports Linux amd64 only. The archives ending in
`_cpu.tar.gz` are unchanged: an untagged CPU-only binary. `go install` also builds the
CPU-only binary. The macOS archives are CPU-only, untested and unsupported for now.

To build the GPU binary yourself on Linux amd64, use
`go install -tags cuda,vulkan github.com/pkieltyka/pbrain/cmd/pbrain@<tag>`. On Linux
arm64, use `-tags cuda`.

## AMD GPUs

`pbrain sync` can embed the `embeddinggemma-2` model on a discrete AMD GPU through
Vulkan. `pbrain devices` lists each such card as `vulkan:<n>`, with its total memory
and estimated headroom when available. `pbrain devices --json` adds a persistent ID,
`vulkan:<uuid>`. The list shows GPU selection order, followed by the CPU. Use
`pbrain sync --device vulkan` to require an AMD card, or `--device vulkan:<uuid>` to pin
one. The Vulkan batch budget uses 30% of estimated free memory up to 1 GiB, or
1 GiB when unknown. `PBRAIN_GPU_MEMORY` can set it directly.

The GPU binary needs neither driver to start. Without an NVIDIA driver and without the
Vulkan loader (`libvulkan.so.1`), `pbrain devices` lists the CPU, and an
`embeddinggemma-2` sync runs on the CPU and says so. Search always runs on the CPU.

A GPU driver may print lines of its own on standard error. Programs that read
`--json` notices from standard error should skip any line that is not a JSON object.

## GPU sync batches

CUDA checks the attached device's workspace estimate when packing batches.
CUDA and Vulkan both retain the 8,192-position aggregate limit. If the accelerator reports a
retry-safe out-of-memory error, sync splits the batch and retries. With automatic
selection, a retry-safe failure on a single passage moves sync to the CPU and
prints a notice.

## Smaller EmbeddingGemma 2 download

Set `semantic.model = "embeddinggemma-2-q8-0"` to use Unsloth's Q8_0 conversion of
Google's EmbeddingGemma 2. Sync downloads a single 310 MB GGUF file and runs on
CPU or CUDA. Search runs on CPU. `bge-small-en-v1.5` remains the default.
[Pinned Unsloth model card and Apache-2.0 license](https://huggingface.co/unsloth/embeddinggemma-2-GGUF/blob/031f0d4b35536f69ab3509d4893c923264fcf253/README.md)
