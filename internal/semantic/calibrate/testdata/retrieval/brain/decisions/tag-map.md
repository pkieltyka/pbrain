---
kind: decision
title: "Tag vocabulary is advisory"
created: 2026-11-21T12:00:00Z
author: agent:fixture-editor
source: "Functional spec §§7.5, 9.1, 10"
status: verified
decided: human:fixture-owner
tags: ["tags","config"]
---

An optional [tags] table maps names to descriptions. Retain unfamiliar tags and emit an undeclared_tag notice. Check warns. If the secret scan flags the tag value, identify the field without repeating the value.
