Semantic search finds items that say a thing in other words. It reads a local index that pbrain sync builds with the project's embedding model. A search never downloads or waits: without an index you still get the exact matches, with a semantic_not_ready notice. With an index, a search first indexes up to 20 new or changed items itself, in about two seconds.

The first pbrain sync on a machine is a person's step. It downloads the model once (about 135 MB for the default, bge-small-en-v1.5; about 1.5 GB for embeddinggemma-2) and then embeds every item: minutes for a small brain, hours for a large one. On 16 cores, 5,000 items of 50 MB took the default about 20 minutes; embeddinggemma-2 took an hour on an NVIDIA GPU and is projected at 15 hours without one. That test text is token-heavy, so real notes may go faster. A stopped run resumes from the last saved checkpoint at the next pbrain sync. Do not start it yourself. Tell the person what the notice said and let them run pbrain sync, or ask before you do. The model_download notice names the model, its size and the host before anything is fetched.

To tell a first index from a catch-up, run pbrain sync --status. It changes nothing. "not on this machine", or a section with nothing indexed, means a first index.

Once an index exists, keeping it current is yours: run pbrain sync after a batch of writes or a pull, or when a semantic_partial notice counts items left out. That run embeds only what changed.

If the notice says semantic search is off, the project turned it off in pbrain.toml. Leave that file as it is.

Results marked semantic were found by meaning alone. Read them before relying on them.
