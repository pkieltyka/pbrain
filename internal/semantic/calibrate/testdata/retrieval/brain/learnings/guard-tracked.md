---
kind: learning
title: "An ignore match alone does not protect personal data"
created: 2026-11-21T12:00:00Z
author: agent:fixture-editor
source: "Functional spec §5.3; brain.md, Exposure guard and display paths"
status: verified
verified: agent:fixture-editor
tags: ["privacy","git"]
---

Check Git’s actual index as well as ignore status before opening the brain. A forced add can put a private path in the repository even while it matches an ignore rule. Shared commands also stop on positive exposure.
