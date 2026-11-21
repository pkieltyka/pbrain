---
kind: learning
title: "Use the file line in an editor and the body offset with get"
created: 2026-11-21T12:00:00Z
author: agent:fixture-editor
source: "Functional spec §§6.2, 7.1–7.2"
status: verified
verified: agent:fixture-editor
tags: ["navigation","output"]
---

Search returns a one-based file line including front matter and a zero-based code-point offset in the body. Both locate the start of the snippet. Use path:line in a file tool or pbrain get with --offset. The body_line field bridges the two coordinates.
