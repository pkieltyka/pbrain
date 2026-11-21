---
kind: decision
title: "Keep personal passage vectors in the personal section"
created: 2026-11-21T12:00:00Z
author: agent:fixture-editor
source: "Functional spec §§5.1, 9.4"
status: verified
decided: human:fixture-owner
tags: ["index","privacy"]
---

The per-user passage cache reuses shared text across worktrees. Personal vectors stay under the personal section’s .pbrain/ and never enter that shared cache.
