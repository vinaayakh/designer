---
id: "005"
title: "How do we search the Knowledge Base locally in Go?"
labels: [wayfinder:research]
status: closed
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

## Resolution

Full findings: branch `research/005-how-do-we-search-the-knowledge-base-locally-in-go`, file `research/005-how-do-we-search-the-knowledge-base-locally-in-go.md`.

- **Recommended: tags/filters plus BM25 keyword search; no embedding model in v1.** Claude Code rewrites and retries queries itself, which supplies the "semantic" part.
- **Measured** (`CGO_ENABLED=0`, cross-compiled for linux/darwin/windows, 2,000 synthetic records of about 1 KB):
  - SQLite FTS5 via `modernc.org/sqlite`: adds about 4.2 MB to the binary, queries take 0.26–8 ms, memory peaks around 32 MB.
  - Hand-written in-memory BM25: adds about 0.04 MB, queries take 0.04–0.5 ms, memory peaks around 34 MB.
  - `ncruces/go-sqlite3`: adds about 6.5 MB.
  - Bleve: adds about 10.5 MB, memory up to 360 MB, and its vector search needs cgo.
- **Transformer embeddings** (MiniLM via ONNX) need cgo plus a separate runtime library, which breaks the single static binary. **Static embeddings** (Model2Vec) could run in pure Go (adds about 8–31 MB; NanoBEIR 0.50 vs BM25 0.45), but we'd have to write the library. The Go vs Rust question does not come back.
- **Whole Knowledge Base in the prompt** (about 100k–800k tokens) works against the token-efficiency goal. A compact catalog tool (ID, title and tags per record) is worth testing in the token spike.
- **Not measured:** search quality for keyword vs semantic (the corpus was synthetic).
- **Decisions raised:** moved into "What is the Knowledge Base record format?" and "How few tokens can the Assistant use? (spike)".
