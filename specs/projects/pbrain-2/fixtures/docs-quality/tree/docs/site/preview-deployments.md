---
layout: fjordgrid
title: Preview deployments
permalink: /handbook/previews/
nav_order: 7
---

Every merge request gets its own copy of the site, built from the branch. A copy stays up for nine days after its last push and is then removed by the sweeper job.

A copy is public but unlisted. Do not put customer data into one: anyone with the link can read it.

To keep a copy longer, add the label "hold" to the merge request. The sweeper skips held copies and reports them every Friday.
