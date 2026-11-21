---
kind: decision
title: "Keep a copied original while any same-section item needs it"
created: 2026-11-21T12:00:00Z
author: agent:fixture-editor
source: "Functional spec §8.6"
status: verified
decided: human:fixture-owner
tags: ["delete","originals"]
---

After deleting a source item, check its valid digest against all items of its section, replaced items included. Remove the original only after the item file is gone and no other item references it. A copy in the other section does not affect the decision.
