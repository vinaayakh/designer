---
title: "Designer: the way to a v1 spec"
labels: [wayfinder:map]
status: open
---

# Designer: the way to a v1 spec

## Destination

An **implementation-ready v1 spec** for a local, AI-assisted architecture diagramming tool. Every major decision is settled so building can start. It comes with a **pilot of about 5 Knowledge Base records** that prove the record format on real content. Targets live in [REQUIREMENTS.md](../REQUIREMENTS.md); vocabulary in [CONTEXT.md](../CONTEXT.md).

## Notes

- **Domain:** a local-only, solo-first diagramming tool. One Go binary, installed with a shell script, serves a React + Excalidraw UI in the browser (the px0 model). AI work is handed off to the user's local **Claude Code** through an MCP server run by the runtime. The app never handles API keys.
- **Every session:** read `REQUIREMENTS.md` and `CONTEXT.md` first. Use the Skill tool for `grilling` + `domain-modeling` on grilling tickets. Keep `REQUIREMENTS.md` (✅/❓/🔬) and `CONTEXT.md` up to date as decisions land.
- **Standing preferences:** performance (fast load, quick on large diagrams) and **token efficiency** are top priorities. The Rough Draft feels like a back-of-the-napkin sketch and is frozen after the first Conversion. Compliance only *reports and suggests*. Changes land only through Proposals the user approves.
- **Tracker conventions (local markdown):** tickets are `wayfinder/tickets/NNN-slug.md` with YAML frontmatter. `status: open|closed`. Claim a ticket by setting `assignee:` first. There is no built-in blocking here, so `blocked_by: [ids]` lists the tickets it waits on. The **frontier** is the open tickets with no assignee whose `blocked_by` tickets are all closed. When a ticket is resolved, add a `## Resolution` section, set `status: closed`, and add a line to Decisions so far. Research findings go on `research/<id>-<slug>` branches.

## Decisions so far

<!-- the index: one line per closed ticket, enough to judge relevance, then open the link for the detail the ticket holds -->

- [How do we run Claude Code as a background agent?](tickets/002-how-do-we-run-claude-code-as-a-background-agent.md): feasible as one long-running `claude -p` process (JSON over stdin/stdout) using our Go MCP server over localhost HTTP, with prompt caching across turns. Domain limits on WebSearch need a hook. `--bare` (needs an API key) is a risk.
- [Which Knowledge Base sources can we use and ship?](tickets/004-which-knowledge-base-sources-can-we-use-and-ship.md): three licence tiers. Public domain (HIPAA rule, NIST) can ship verbatim. OWASP/AWS/Azure Architecture Center can ship with attribution (share-alike). SOC 2, CIS, Azure WAF and the HIPAA service lists are paraphrase or link only. AICPA explicitly objects to its content in LLM knowledge bases.
- [Can Excalidraw be our canvas?](tickets/001-can-excalidraw-be-our-canvas.md): yes. `@excalidraw/excalidraw` (MIT) supports custom data per element, selection events and undoable changes from code, and can render a clean, non-sketchy look. About 350–420 KB gzipped; fonts must ship locally. Speed at our sizes not yet measured (spike ticketed). tldraw (paid licence) and React Flow (no freehand) rejected.
- [How do we draw the Production Diagram?](tickets/003-how-do-we-draw-the-production-diagram.md): ELK.js recommended for automatic layout with nested groups (about 462 KB, loaded only when needed). Groups are plain rectangles because Excalidraw frames can't nest. AWS/Azure icon terms only cover use in diagrams and documentation, not shipping inside an app, so how to ship them needs a decision.
- [How do we search the Knowledge Base locally in Go?](tickets/005-how-do-we-search-the-knowledge-base-locally-in-go.md): tags/filters plus BM25 keyword search, no embedding model in v1 (Claude Code rewrites queries itself). Pure Go keeps the single binary: SQLite FTS5 adds about 4 MB, a hand-written BM25 about 0.04 MB, both with sub-10 ms queries. Go stays.
- [What is the diagram data model?](tickets/006-what-is-the-diagram-data-model.md): one `.excalidraw` file per diagram, with meaning stored in each element's `customData` and the graph calculated when needed (ADR 0001). The Node ID is `element.id`. Two type layers: Component Kind and Service. Production Group membership is an explicit parent link set on drop, with mismatches flagged; Rough Draft membership comes from position. The AI reads a compact indented format with stable short aliases.
- [How few tokens can the Assistant use? (spike)](tickets/012-how-few-tokens-can-the-assistant-use-spike.md): the compact format is about 49× smaller than raw JSON. Brief, selected nodes and the first diagram go in the prompt, and later turns get only changes. About 6 tools; Knowledge Base access by search plus `get_record`. One long-running process per Project; replace the system prompt (−38% input). Rule of thumb: tokens ≈ context × round trips.

## Not yet specified

- **Comparing versions:** how two Production Diagram versions are compared side by side (visual diff vs change list), and how far back history goes.
- **Export:** PNG/SVG for sure. Whether the Design Rationale and Gap Report export as markdown/PDF.
- **Install and update:** the `curl | sh` script, self-update (`designer --update`), supported operating systems, checking that Claude Code is installed.
- **Project folder layout:** the file layout inside a project folder (brief, frozen draft, diagram versions, rationale, reports, chat history?).
- **Keeping the Knowledge Base current:** how Records get refreshed after v1 ships, and how a Record's version shows up in a Gap Report.
- **Chat history:** whether Assistant conversations are saved per Project, and how they relate to diagram versions.

## Out of scope

- Infrastructure-as-code (Terraform/CDK) generation: not in v1.
- Team collaboration and real-time co-editing: solo-first v1.
- Hosted/SaaS deployment: local only.
- Writing the full Knowledge Base: this effort only pilots the format.
- New Rough Drafts in an existing Project (sketching an extension, converted as a Proposal): v2.
- CLI agents other than Claude Code: v1 is Claude Code only.
- GCP target: v1 is Generic + AWS + Azure (the data catalog leaves room for it).
- User-extensible Knowledge Base (org-specific standards): v1 ships a read-only, curated Knowledge Base.
