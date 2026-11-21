# Operator's initial notes (read only AFTER writing your own round-1 file)

These are one view to challenge, not a direction.
1. Vocabulary mismatch: all-clauses-must-match, no stemming -> agent task-phrased queries often return nothing. Consider BM25-style relevance (more matched words rank higher), light stemming, keep "no fallback to non-matches".
2. Declared tag vocabulary in pbrain.toml: [tags] name = {description, aliases, paths}. pbrain tags lists vocabulary with counts (cheap routing map); aliases expand search; undeclared tag warns on write and in check; paths map code areas to tags (search --for <file>).
3. Junk control at write time: add warns on similar existing items; skill: one fact per item, title states the fact; check lists old provisional observations for pruning.
4. Long sources crowding out short learnings: length normalisation; first-party vs source ordering at equal match, or sources only on request.
5. Task-shaped retrieval: pbrain context "<task>" returning one budgeted bundle.
6. Evaluation: golden query set in CI measuring hit quality and characters spent. Architecture: scan vs rebuildable index in .pbrain/, a seam for semantic search/qmd later, a synthetic scale test.
