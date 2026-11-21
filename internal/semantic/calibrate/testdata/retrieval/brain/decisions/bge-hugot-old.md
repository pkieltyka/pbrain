---
kind: decision
title: "Use hugot for the initial bge-small runtime"
created: 2026-11-18T12:00:00Z
author: agent:fixture-editor
source: "Historical bge-small adapter; retained fallback design"
status: verified
decided: human:fixture-owner
tags: ["runtime","models"]
---

The original spike measured the pure-Go backend. Its design used an in-memory filesystem and enabled SIMD at build time.
