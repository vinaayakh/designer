---
id: "007"
title: "What is the Knowledge Base record format?"
labels: [wayfinder:grilling]
status: closed
assignee: vinu (with claude)
blocked_by: ["004", "005", "014"]
parent: map
---

# What is the Knowledge Base record format?

## Question

What fields does a Knowledge Base Record have (ID, kind: design/security/compliance, regime, clouds, applicable component types, which Design Brief conditions trigger it, summary, body, how to check it against a diagram, sources with citations, version/date), and how are Records stored in the repo and built into the binary?

Also decide (from "How do we search the Knowledge Base locally in Go?"):
- **Search engine:** SQLite FTS5 via `modernc.org/sqlite` (stemming, snippets, richer queries; adds about 4 MB) or a hand-written in-memory BM25 (adds about 0.04 MB, fastest; we write the tokenizer and stemmer).
- **Search results:** snippets plus IDs, or whole Records?
- **Test set:** build about 30 test queries now, so semantic search can later be judged with data?

Constraints from "What are the licensing rules for the Knowledge Base and icons?":
- Each entry in `sources[]` carries a **`licence_tier`** (`public-domain` / `attribution` / `facts-only`) and the attribution text, if any.
- A **CI check** fails any Record whose `facts-only` source shares a long verbatim run (start at 12+ words) with that source's text. Decide where the reference copies of the source texts live for this check (not shipped).
- SOC 2 Records are keyed by criterion ID (`CC6.1`) with our own description. No CIS fields.
- HIPAA Security Rule text ships verbatim (public domain) so the Assistant needn't fetch hhs.gov/ecfr.gov. Records carry an as-of date because the rule is being rewritten.

## Resolution

Decided with vinu on 2026-10-10.

**Record types and size**
- Two types:
  - **`control`**: one item on a compliance checklist. It drives the Gap Report.
  - **`practice`**: a design or security practice, tagged `domain: design | security`. It drives Conversion and the Design Rationale.
- NIST and OWASP are *sources* that practices cite, not Record sets of their own.
- **What one Record covers:**
  - SOC 2: one Record per **criterion** (`CC6.1`).
  - HIPAA: one Record per **implementation specification** (`§164.312(a)(2)(iv)`, marked required or addressable). A standard with no specifications gets one Record of its own.
  - Practices: one Record per named practice or pattern.
  - That puts the v1 Knowledge Base at roughly 150–250 Records.

**Storage**
- One markdown file per Record with YAML frontmatter, at `kb/records/<regime|practice>/<slug>.md`. The path mirrors the ID.
- Records are embedded with `go:embed`, and the search index is built in the background at startup.

**IDs**
- Lowercase, starting with the regime or type, and stable forever: `hipaa.164-312-a-2-iv`, `soc2.cc6-1`, `practice.encryption-at-rest`.
- Retired IDs are listed in `kb/retired.yaml`, and CI blocks reusing them.

**Fields**

```yaml
id: hipaa.164-312-a-2-iv
type: control                    # control | practice
domain:                          # practices only: design | security
regime: hipaa                    # controls only: soc2 | hipaa
ref: "§164.312(a)(2)(iv)"        # official ID, controls only
obligation: addressable          # HIPAA only: required | addressable
title: Encryption and decryption of ePHI
summary: <authored, ≤25 words; returned by search>
aliases: [ePHI at rest, encryption at rest, KMS, …]   # synonyms in place of stemming
coverage: checkable              # checkable | partial | process
applies_to:                      # every listed key must match; omitted = any
  kinds: ["data.*", "storage.*"] # Component Kind IDs, glob allowed
  data_classes: [PHI]
  clouds: [generic, aws, azure]
  brief: { compliance: [hipaa] } # Brief field → accepted answers (any of); unknown = applies
satisfied_by: [practice.encryption-at-rest]   # controls only; reverse link `satisfies` built at load
sources:
  - title, url, as_of
    licence_tier: public-domain | attribution | facts-only
    attribution: <required when tier = attribution>
revised: 2026-10-10
```

- **Body:** fixed sections, in order:
  - `## Requirement` (controls) or `## Practice` (practices)
  - `## How to check on a diagram`
  - `## Suggested fixes`: prose only, never Proposal operations
  - `## Trade-offs` (practices only)
- Bodies have a soft cap of about 350 tokens, enforced as a CI warning.

**Who decides whether a diagram meets a Record:** the AI, not runtime rules (v1).
- The runtime filters by `applies_to`, so the Records that apply are found by lookup, not by search.
- Records marked `coverage: process` go straight to "outside what the diagram can show" without any AI judgement.
- Machine-checked rules are possible later. Nothing is built for them now, because Nodes carry no security attributes.

**Search**
- A **hand-written BM25** in Go, not SQLite FTS5. Fields are weighted title > aliases > summary > body, with exact filters on tags. It sits behind the `search_kb` interface, so switching to FTS5 later stays local.
- `search_kb` returns the **top 8 by default, as ID, title, summary and tags** (about 40 tokens per hit). The AI then calls `get_record` for the full Record.

**Versioning**
- The Knowledge Base gets a date version (`kb-2026.11`), built into the binary and stamped on every Gap Report.
- Each Record carries `revised`, and each source carries `as_of`. No per-Record version numbers.

**Copy check sources**
- Public-domain and CC sources are committed under `kb/sources/` (never shipped in the binary) and checked in CI.
- Restricted sources (SOC 2, Azure Well-Architected) live in a gitignored local cache on the author's machine. Their check runs as a pre-commit step, and CI skips it with a notice.

**Search test set**
- `kb/eval/queries.yaml`: each entry is `{query, expect: [record ids]}`. CI reports the hit rate in the top 5.
- The ~30 queries themselves are written in "Write the 5 pilot Knowledge Base records".
