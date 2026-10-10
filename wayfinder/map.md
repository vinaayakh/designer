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

## Not yet specified

- **Comparing versions:** how two Production Diagram versions are compared side by side (visual diff vs change list), and how far back history goes.
- **Export:** PNG/SVG for sure. Whether the Design Rationale and Gap Report export as markdown/PDF.
- **Install and update:** the `curl | sh` script, self-update (`designer --update`), supported operating systems, checking that Claude Code is installed.
- **Project folder layout:** the file layout inside a project folder (brief, frozen draft, diagram versions, rationale, reports, chat history?).
- **Cloud service catalogs:** contents and structure of the Generic/AWS/Azure catalogs (services, categories, generic equivalents, HIPAA eligibility). Depends on the data model and the icon research.
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
