---
kind: decision
title: "Score an ordinary SQLite table with vec_distance_cosine"
created: 2026-11-21T12:00:00Z
author: agent:fixture-editor
source: "Architecture §§8.7 and 16; owner decision 22"
status: verified
decided: human:fixture-owner
tags: ["index","storage"]
replaces: ["flat-vectors-old"]
---

Store passage vectors per section. Apply eligibility first, then compute exact cosine distance with sqlite-vec. A vec0 nearest-neighbour query limits k, whereas the product needs every item above its calibrated threshold.
