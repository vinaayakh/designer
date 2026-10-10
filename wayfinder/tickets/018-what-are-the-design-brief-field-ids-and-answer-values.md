---
id: "018"
title: "What are the Design Brief field IDs and answer values?"
labels: [wayfinder:grilling]
status: open
assignee:
blocked_by: []
parent: map
---

# What are the Design Brief field IDs and answer values?

## Question

Knowledge Base Records filter on Brief answers (`applies_to.brief: { compliance: [hipaa] }`), and Conversion and the Assistant read the Brief as text. What are the stable machine IDs for the 14 Design Brief questions from "What does the Design Brief interview look like?" (target cloud, compliance, CAP, users, latency, throughput, read/write bias, availability, data volume, RPO, RTO, regions/residency, budget, team maturity)? What is each one's fixed set of answer values, including `unknown`, and `none` versus unanswered for compliance? Which fields can Records filter on, and how is the Brief stored in the project folder?

_From "Write the 5 pilot Knowledge Base records":_ the pilot used provisional `compliance: [soc2|hipaa]` and `read_write_bias: [read-heavy]`, and unknown answers make a Record apply. Recommendation to confirm here: `brief` is a hard filter for controls but only a ranking hint for practices.
