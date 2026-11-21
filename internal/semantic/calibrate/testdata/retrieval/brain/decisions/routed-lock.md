---
kind: decision
title: "The item owns the write lock"
created: 2026-11-21T12:00:00Z
author: agent:fixture-editor
source: "Functional spec §§5.2, 8.3, 8.6"
status: verified
decided: human:fixture-owner
tags: ["concurrency","audience"]
---

For verify and delete, --personal widens lookup. Lock the section containing the resolved item. Verifying a shared item through the combined view edits the shared file under the shared writer lock.
