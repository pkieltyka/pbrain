# Generate model fs-verity pins

Run the generator from the repository at the revision being qualified.
It verifies the size and SHA-256 of each final file against the release
registry before hashing the private verified bytes. Output columns are
the name, size, release SHA-256 and fs-verity file digest.

```sh
go run ./tools/model-verity-pins/testdata/main.go -model embeddinggemma-2-q8-0 -dir /absolute/path/to/q8
```

The checked-in generator implements Linux fs-verity version 1 with SHA-256,
4,096-byte blocks and no salt. This includes hashing the file descriptor
structure in the format, which binds the Merkle root to the length and
parameters. Use the same command with `embeddinggemma-2-ud-q6-k-xl` and
`embeddinggemma-2`. For the Google model, use the final pbrain pack.
[Linux format specification](https://docs.kernel.org/filesystems/fsverity.html#file-digest-computation)

Run the release pin check with all three local fixtures. The opted-in test
fails when one fixture is missing or any digest disagrees with its row.
The ordinary test checks that each release row declares its pins.

```sh
PBRAIN_TEST_MODEL_VERITY=1 \
PBRAIN_TEST_GEMMA2_PACK=/absolute/path/to/google-pack \
PBRAIN_TEST_GEMMA2_Q8=/absolute/path/to/q8 \
PBRAIN_TEST_GEMMA2_Q6=/absolute/path/to/q6 \
go test ./cmd/pbrain -run '^TestModelVerityPins$' -v
```

To cross-check the implementation against the kernel on a filesystem
that supports fs-verity, run the native test with
`PBRAIN_REQUIRE_FS_VERITY=1 go test ./internal/embed -run '^TestNativeVerity$'`.
Set `TMPDIR` to a directory on that filesystem if needed.
