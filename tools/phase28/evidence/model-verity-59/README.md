# Model verification cache measurements (#59)

The source revision is `4e949dce89cc10bc7f0783792b92bb7da79e915d`.
The baseline is `d8384b144a23b30e6127574c048033dcc7944efe`.
Both used goinfer v0.6.4, Go 1.27.3, CGO disabled and `-trimpath`.

The archive contains the shell-free Python driver, the source overlay generator,
release-pin validation, fixed release pins and the raw records from pax. Run
`instrument.py REPO OVERLAY_DIRECTORY base|head` on the corresponding
source tree after making the overlay directory. Build `./cmd/pbrain` with
`go build -overlay OVERLAY_DIRECTORY/overlay.json`. The overlays bracket the
production source model phases and record kernel-assisted verification hits on
the candidate. They only emit measurement data when the runner asks for it.

Put the two binaries beside the scripts as `pbrain-base` and `pbrain-head`.
For the raw controls, build untagged test binaries of
`./internal/embed/bge` and `./internal/embed/gemma2` as `bge.test` and
`gemma2.test`. Run `python3 rule3.py NEW_OUTPUT_DIRECTORY bge google q8` on
pax with the locally provisioned release fixtures. The script uses pre-staged
final model weights and normal sync to create and then reuse the test index.
No model weights or indexes are included in this archive.

Each row's `samples.jsonl` contains all 20 observations, including the
strict adapter control. The search output files record ranked IDs and
notices, while the sample records report query-vector SHA-256 for equality.
