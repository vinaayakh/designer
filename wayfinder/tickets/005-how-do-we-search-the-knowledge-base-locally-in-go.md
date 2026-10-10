---
id: "005"
title: "How do we search the Knowledge Base locally in Go?"
labels: [wayfinder:research]
status: open
assignee: claude (research subagent)
blocked_by: []
parent: map
---

# How do we search the Knowledge Base locally in Go?

## Question

How should the Go runtime search the built-in Knowledge Base locally, with no API key and no external service, while keeping the binary small and fast?

Compare:
- Keyword/full-text search: SQLite FTS5 (pure-Go drivers, e.g. `modernc.org/sqlite`, vs cgo), Bleve, a simple in-memory BM25.
- Small local embedding models running in Go (e.g. ONNX runtime bindings, pure-Go options), with embeddings computed at build time and only the query embedded at runtime: binary size, memory, speed.
- Whether a corpus of about 500–2,000 short, tagged records even needs semantic search, or whether tags/filters (cloud, regime, category) plus keyword search are enough.
- Note: if a built-in embedding model is recommended, the Go vs Rust question comes back up (see REQUIREMENTS.md).

## Context

Findings: branch `research/005-how-do-we-search-the-knowledge-base-locally-in-go`, file `research/005-how-do-we-search-the-knowledge-base-locally-in-go.md`.
