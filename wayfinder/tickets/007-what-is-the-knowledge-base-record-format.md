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
