Semantic search finds items that say a thing in other words. It reads a local index that pbrain index builds with the project's embedding model. A search never downloads, builds or waits: without an index you still get the exact matches, with a semantic_not_ready notice.

The first pbrain index on a machine is a person's step. It downloads the model once (about 1.5 GB for the default, embeddinggemma-2) and then embeds every item. With the default model on four cores that ran at about 3 MB of items an hour: minutes for a small brain, a projected 15 hours for 5,000 items of 50 MB. That test text is token-heavy, so real notes may go faster. A stopped run resumes from the last saved checkpoint at the next pbrain index. Do not start it yourself. Tell the person what the notice said and let them run pbrain index, or ask before you do. The model_download notice names the model, its size and the host before anything is fetched.

To tell a first index from a catch-up, run pbrain index --status. It changes nothing. "not on this machine", or a section with nothing indexed, means a first index.

Once an index exists, keeping it current is yours: run pbrain index after a batch of writes or a pull, or when a semantic_partial notice counts items left out. That run embeds only what changed.

If the notice says semantic search is off, the project turned it off in pbrain.toml. Leave that file as it is.

Results marked semantic were found by meaning alone. Read them before relying on them.
