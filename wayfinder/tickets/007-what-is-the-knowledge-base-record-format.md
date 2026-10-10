---
id: "007"
title: "What is the Knowledge Base record format?"
labels: [wayfinder:grilling]
status: open
assignee:
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
