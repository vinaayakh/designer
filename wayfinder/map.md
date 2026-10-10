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
- [Is Excalidraw fast enough? (spike)](tickets/015-is-excalidraw-fast-enough-spike.md): go. Smooth at 500 and 2,000 elements and usable in about 0.4 s. Pinned `@excalidraw/excalidraw@0.18.0-143b5b6` (@next) with a lazy shim for mermaid. The frozen Rough Draft stays read-only with our own click handling. When the app moves Nodes it must recalculate their arrows.
- [What does the Design Brief interview look like?](tickets/009-what-does-the-design-brief-interview-look-like.md): a local wizard (no Assistant, no tokens), one question per screen, with the brief always visible beside it. The 3 required questions come first and gate the Rough Draft, then you can start sketching or answer more, and skipped questions stay `unknown`. CAP is asked as a network-split scenario with CP and AP only.
- [How are Proposals shown and reviewed?](tickets/010-how-are-proposals-shown-and-reviewed.md): changes are drawn as an overlay on the canvas. The user steps through them (accept or skip), or jumps to a full review with one Accept. Parts can be accepted on their own, and the runtime works out dependencies. Accepted changes become one version with one Undo. The canvas is locked while a Proposal is waiting. Replying creates a new revision. Draft `submit_proposal` payload: ops with aliases and `+new` references, no coordinates.
- [How is the Claude Code session configured?](tickets/013-how-is-the-claude-code-session-configured.md): a static system prompt, with Project context in messages. Trusted domains are enforced by WebFetch domain rules plus an HTTP hook to our server that makes WebSearch use `allowed_domains`. The process starts when the sidebar opens, stops after 15 min idle, and resumes with `--resume`; after a crash it resumes once, then starts fresh. MCP runs over localhost HTTP with a bearer token. The model is a per-install setting and can be switched mid-session. Isolated from the user's own Claude Code config (`--setting-sources ""`, empty working directory). `--bare` risk accepted, with a version check.
- [What are the licensing rules for the Knowledge Base and icons?](tickets/014-what-are-the-licensing-rules-for-the-knowledge-base-and-icons.md): free and open source but used at work, so it's treated as commercial. The Knowledge Base is written in-house with no share-alike (ADR 0002): public domain verbatim, CC BY adapted, CC BY-SA and restricted sources as facts and IDs in our own words, enforced by a CI check for copied text. SOC 2 written in our own words; CIS out. No legal review until it's sold. Allowlist of 12 fetch domains, with HIPAA served from the built-in copy. AWS/Azure icons download on first use after the user agrees to the terms, with generic icons as the fallback and official icons locked against edits.
- [What is the Knowledge Base record format?](tickets/007-what-is-the-knowledge-base-record-format.md): one markdown file per Record with YAML frontmatter, of two types: `control` (one SOC 2 criterion or HIPAA implementation specification) and `practice`. Fields cover when a Record applies (Kinds, Data Classes, cloud, Brief), how much a diagram can show (`coverage`), links from controls to practices (`satisfied_by`), and sources with licence tiers. The AI judges compliance from the Records that apply, with no runtime rules. Search is a hand-written BM25 returning ID, title and summary. The Knowledge Base has a date version.

## Not yet specified

- **Comparing versions:** how two Production Diagram versions are compared side by side (visual diff vs change list), and how far back history goes.
- **Export:** PNG/SVG for sure. Whether the Design Rationale and Gap Report export as markdown/PDF.
- **Install and update:** the `curl | sh` script, self-update (`designer --update`), supported operating systems, checking that Claude Code is installed.
- **Project folder layout:** the file layout inside a project folder (brief, frozen draft, diagram versions, rationale, reports, chat history?).
- **Keeping the Knowledge Base current:** how Records get refreshed after v1 ships, and how a Record's version shows up in a Gap Report.
- **Editing the Design Brief later:** answering or changing brief questions from Project settings after sketching or Conversion has started. Does a change mark the current Production Diagram as stale, or suggest a Proposal?
- **More than one Proposal at once:** can the Assistant have several Proposals waiting (e.g. one per Gap Report item), or only one at a time given the canvas lock? How are they queued?
- **Showing usage and cost:** whether to show the user the per-session token cost Claude Code reports (`total_cost_usd`), and where.
- **App licence:** Designer is open source, but its own licence (MIT, Apache-2.0, …) isn't chosen yet. The Knowledge Base carries the same licence (ADR 0002).
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
- CIS Controls and Benchmarks: their NonCommercial-NoDerivatives licence rules them out, and v1 covers SOC 2 and HIPAA only ([What are the licensing rules for the Knowledge Base and icons?](tickets/014-what-are-the-licensing-rules-for-the-knowledge-base-and-icons.md)).
