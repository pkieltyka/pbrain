Semantic search finds items that say a thing in other words. It reads a local index made by pbrain sync. Without an index you get exact matches and a semantic_not_ready notice. Search never downloads. CPU catch-up can cover up to 20 changed items and 20 passages in about two seconds. An fp16 CUDA index needs a sync for changes.

The first pbrain sync on a machine is a person's step. It downloads the model once (310 MB for the default embeddinggemma-2-q8-0, about 1.5 GB for Google's embeddinggemma-2) and embeds every item. On a GPU a large brain takes minutes to an hour. On CPUs alone it can take many hours. A stopped run resumes from the last checkpoint. Do not start it yourself. Relay the notice and let the person run sync, or ask first. The model_download notice names the model, size and host before fetching.

The GPU archives default to fp16 for both Gemma models on NVIDIA compute capability 8.0 or newer. Run sync after upgrading to rebuild the index and reuse the model files. Set semantic.cuda_precision = "fp32" to keep ordinary CUDA indexing.

To tell a first index from a catch-up, run pbrain sync --status. It changes nothing. "not on this machine", or a section with nothing indexed, means a first index.

Once an index exists, keeping it current is yours: run pbrain sync after a batch of writes or a pull, or when a semantic_partial notice counts items left out. That run embeds only what changed.

If the notice says semantic search is off, the project turned it off in pbrain.toml. Leave that file as it is.

Results marked semantic were found by meaning alone. Read them before relying on them.
