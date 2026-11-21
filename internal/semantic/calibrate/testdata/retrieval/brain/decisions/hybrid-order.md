---
kind: decision
title: "Exact results keep their place in a hybrid page"
created: 2026-11-21T12:00:00Z
author: agent:fixture-editor
source: "Functional spec §7.1; owner BM25 decision"
status: verified
decided: human:fixture-owner
tags: ["search","ranking"]
---

With both optional search modes enabled, show exact hits first in their existing order, then semantic-only hits, then qualifying BM25 partials. Deduplicate by item. A model-free partial result says how many distinct clauses it matched.
