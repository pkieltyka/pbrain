---
kind: note
title: "What semantic_partial counts"
created: 2026-11-21T12:00:00Z
author: agent:fixture-editor
source: "Functional spec §§7.1, 9.4"
status: provisional
tags: ["index","search"]
---

When at least one index is usable, search compares the eligible current content that it covers and reports how many visible items it could not compare. A changed body cannot be scored through its stale vector. If no index can be used, semantic_not_ready accompanies lexical results.
