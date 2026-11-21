---
kind: decision
title: "A dangling_link is a check warning"
created: 2026-11-21T12:00:00Z
author: agent:fixture-editor
source: "Owner batch 4; functional spec §§4.2, 9.1"
status: verified
decided: human:fixture-owner
tags: ["graph","check"]
---

Deleting an item can leave a replaces or relates target missing. A missing target points to nothing on reads. Report one warning, including for a handwritten link; that finding alone leaves check's exit status at zero.
