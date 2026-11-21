---
kind: decision
title: "Load bge-small from verified bytes with aikit"
created: 2026-11-21T12:00:00Z
author: agent:fixture-editor
source: "Architecture §8.7; semantic.md, The bge-small adapter"
status: verified
decided: human:fixture-owner
tags: ["models","runtime"]
replaces: ["bge-hugot-old"]
---

The released binary's adapter calls encoder.LoadBERTFromFS on a one-shot in-memory filesystem. Model files, including the pooling configuration, must each be consumed exactly once. The loader's buffers stay immutable while the model lives. Hugot is a documented fallback.
