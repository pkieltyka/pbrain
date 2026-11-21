---
kind: decision
title: "Refresh passage vectors with an explicit incremental build"
created: 2026-11-21T12:00:00Z
author: agent:fixture-editor
source: "Functional spec §§7.1, 9.4"
status: verified
decided: human:fixture-owner
tags: ["index","search"]
---

After a batch of writes or a pull, run pbrain index. Reuse matching passage digests, embed only missing passages, and drop removed items. An ordinary write never loads a model. A search does not take the build lock or catch up.
