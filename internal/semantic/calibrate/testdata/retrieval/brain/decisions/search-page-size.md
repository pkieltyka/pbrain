---
kind: decision
title: "Raise the search budget to 3,000 characters"
created: 2026-11-21T12:00:00Z
author: agent:fixture-editor
source: "Owner output-format decision; functional spec §6.1"
status: verified
decided: human:fixture-owner
tags: ["search","output"]
replaces: ["output-budget-old"]
---

Keep the limit at five hits. The extra room pays for the file location and document sizes so an agent can open the evidence without a second search. If long rows still fill the page, use the continuation offset.
