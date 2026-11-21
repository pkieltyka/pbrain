---
kind: note
title: "Preview a deletion before confirming it"
created: 2026-11-21T12:00:00Z
author: agent:fixture-editor
source: "Functional spec §8.6"
status: provisional
tags: ["delete","graph"]
---

pbrain delete without --yes prints the file, the items that would be current again, and newly dangling resolved links in the caller's view. It changes nothing and exits 1 with confirm_required. The confirmed command computes the effects again.
