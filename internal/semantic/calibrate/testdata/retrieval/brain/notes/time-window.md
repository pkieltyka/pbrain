---
kind: note
title: "Time filters use created and local calendar days"
created: 2026-11-21T12:00:00Z
author: agent:fixture-editor
source: "Functional spec §7.1"
status: provisional
tags: ["clock","search"]
---

A bare date, today or yesterday uses the machine’s TZ. --until includes the named day up to the following local midnight. Filter before counting and applying the page limit. With a time bound, omit items whose created time is unknown.
