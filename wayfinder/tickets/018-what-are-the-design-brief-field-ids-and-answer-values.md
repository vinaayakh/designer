---
id: "018"
title: "What are the Design Brief field IDs and answer values?"
labels: [wayfinder:grilling]
status: closed
assignee: vinu (with claude)
blocked_by: []
parent: map
---

# What are the Design Brief field IDs and answer values?

## Question

Knowledge Base Records filter on Brief answers (`applies_to.brief: { compliance: [hipaa] }`), and Conversion and the Assistant read the Brief as text. What are the stable machine IDs for the 14 Design Brief questions from "What does the Design Brief interview look like?" (target cloud, compliance, CAP, users, latency, throughput, read/write bias, availability, data volume, RPO, RTO, regions/residency, budget, team maturity)? What is each one's fixed set of answer values, including `unknown`, and `none` versus unanswered for compliance? Which fields can Records filter on, and how is the Brief stored in the project folder?

_From "Write the 5 pilot Knowledge Base records":_ the pilot used provisional `compliance: [soc2|hipaa]` and `read_write_bias: [read-heavy]`, and unknown answers make a Record apply. Recommendation to confirm here: `brief` is a hard filter for controls but only a ranking hint for practices.

## Resolution

Decided with vinu, 2026-10-10.

**Conventions:** field IDs in snake_case, values in lowercase kebab-case (matches the pilot and the Kind IDs).

**Fields (15).** The prototype's "Regions & residency" question is split into `regions` and `residency`, asked on one wizard screen, because "multi-region and EU-only" is a real answer.

| Field | Values | |
|---|---|---|
| `cloud` | `generic` `aws` `azure` | required |
| `compliance` | list of `soc2` `hipaa`, or `[none]` (exclusive) | required |
| `cap` | `cp` `ap` | required |
| `users` | `lt-1k` `1k-100k` `100k-1m` `1m-plus` | |
| `latency` | `seconds` `lt-300ms` `lt-50ms` | |
| `throughput` | `lt-100` `100-1k` `1k-10k` `10k-plus` (req/s) | |
| `read_write_bias` | `read-heavy` `balanced` `write-heavy` | |
| `availability` | `99` `99.9` `99.95` `99.99` | |
| `data_volume` | `gb-slow` `tb-steady` `tb-fast` `pb` | |
| `rpo` | `1d` `1h` `minutes` `zero` | |
| `rto` | `1d` `hours` `minutes` `seconds` | |
| `regions` | `single` `multi` | |
| `residency` | `none` `eu` `us` | |
| `budget` | `cost-first` `balanced` `performance-first` | |
| `ops_maturity` | `small-team` `ops-team` `sre` | |

- Every optional field can also be `unknown`. There is no `other` residency in v1.
- **Compliance "None" is the explicit value `[none]`**, never an empty list. Compliance is required, so an unanswered value never reaches the AI or the Records.
- **`notes`**: optional free text, ≤ 500 characters, on the last wizard screen and in Project settings. The AI gets it word for word, and Records never filter on it.

**Storage**
- `brief.yaml` at the project root, starting with `schema: 1`. **Every field is always written**, with `unknown` spelled out, so a git diff shows the whole Brief.
- A **brief schema** built into the binary (fields, order, required flag, values, wizard labels and questions, short names). It's the one source for the wizard, the runtime validator and the Knowledge Base CI.

**Records filtering on the Brief**
- Any field may appear in `applies_to.brief` as a list of accepted values. There are no comparison operators (e.g. `availability: [99.9, 99.95, 99.99]`). CI rejects unknown fields or values.
- `unknown` always matches.
- **Controls:** `brief` is a hard filter. **Practices:** `brief` is only a ranking hint, boosting matches in `search_kb` without hiding anything (confirms the pilot's recommendation).

**What the AI sees** (first message and the Conversion message, about 70 tokens):
```
brief: cloud=aws compliance=hipaa cap=ap users=1k-100k rw=read-heavy availability=99.9 rpo=minutes rto=hours residency=us
unknown: latency throughput data_volume regions budget ops_maturity
notes: "Must integrate with on-prem SAP via VPN"
```
Known fields first, then unknown fields by name so the AI states an assumption for each. Short names (`rw`) come from the schema.

Not decided here: changing answers after sketching or Conversion has started (the "Editing the Design Brief later" fog), and the rest of the project folder layout.
