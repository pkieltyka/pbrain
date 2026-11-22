Semantic search finds items that say a thing in other words. It reads a local index that pbrain sync builds with the project's embedding model. A search never downloads or waits: without an index you still get the exact matches, with a semantic_not_ready notice. With an index, a search first indexes up to 20 new or changed items itself, in about two seconds.

The first pbrain sync on a machine is a person's step. It downloads the model once (about 135 MB for the default bge-small-en-v1.5, about 1.5 GB for Google's embeddinggemma-2, 310 MB for Unsloth's conversion embeddinggemma-2-q8-0) and embeds every item. A small brain takes minutes. On 50 MB of token-heavy text, the default took 20 minutes on 16 cores. Google's pack took an hour on an NVIDIA GPU, with a CPU projection of 15 hours. A stopped run resumes from the last saved checkpoint. Do not start it yourself. Tell the person what the notice said and let them run pbrain sync, or ask before you do. The model_download notice names the model, size and host before fetching.

To tell a first index from a catch-up, run pbrain sync --status. It changes nothing. "not on this machine", or a section with nothing indexed, means a first index.

Once an index exists, keeping it current is yours: run pbrain sync after a batch of writes or a pull, or when a semantic_partial notice counts items left out. That run embeds only what changed.

If the notice says semantic search is off, the project turned it off in pbrain.toml. Leave that file as it is.

Results marked semantic were found by meaning alone. Read them before relying on them.
