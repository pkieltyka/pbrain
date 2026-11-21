# EmbeddingGemma 2 reference

These are the 12 documents and 10 queries from the runtime comparison, in that order, against google/embeddinggemma-2 at revision `f9972ac22e696a105c482202646979e33e4b5a1e`. The reference is independent of goinfer. It uses the checkpoint tokenizer and sentence-transformers on CPU in float32. Each row contains the input, role, BOS/EOS-framed token IDs and the 768-dimensional normalized vector (rounded to nine decimal places). The tests also compare the first 512 dimensions by cosine.

For the raw reference, encode the checkpoint's role prompt plus the text with special-token recognition on. For the adapter reference, use the Hugging Face tokenizer with `encode_special_tokens=True` (literal marker text), `add_special_tokens=False`, and frame the ordinary IDs with BOS and EOS. For overlong text, apply the 2,048-token preparation rule in semantic.md. Pass these IDs and an all-ones attention mask through the same sentence-transformers model, in evaluation mode, and normalize the sentence embedding. Only the 2,294-token document and the literal-marker document differ from the raw reference. Their rows have separate `prepared_ids` and `prepared_vector` fields.

Set `PBRAIN_TEST_GEMMA2_SOURCE` to the directory containing the four original files for the deterministic-derive and tokenizer tests. Set `PBRAIN_TEST_GEMMA2_PACK` to a directory containing the four derived files for parity, buffer-ownership and cut tests. These tests use local files only, and skip unless `PBRAIN_ALLOW_GEMMA2=1`, the team switch for EmbeddingGemma 2 workloads, which only the owner turns on. To measure verification, open and first query together, run the benchmark with four threads and a warm file cache:

```sh
GOMAXPROCS=4 CGO_ENABLED=0 go test -p 2 ./internal/embed/gemma2 -run '^$' -bench BenchmarkGemmaFirstQuery -benchtime=1x -count=3
```
