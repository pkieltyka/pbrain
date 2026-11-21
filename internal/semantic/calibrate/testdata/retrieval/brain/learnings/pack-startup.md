---
kind: learning
title: "Prebuilding the tokenizer removes repeated JSON work"
created: 2026-11-21T12:00:00Z
author: agent:fixture-editor
source: "goinfer-tokenizer-spike.md, Bottom line and How it was checked"
status: verified
verified: agent:fixture-editor
tags: ["tokenizer","performance"]
---

The EmbeddingGemma 2 spike reduced tokenizer initialization from 441–502 ms to 24–26 ms on the test laptop, with identical token IDs. Building the pack is a one-time derivation from pinned model files. The measurement covers the tokenizer, not the whole encoder.
