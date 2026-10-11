---
title: "Designer: the way to a Document Pipeline spec"
labels: [wayfinder:map]
status: open
---

# Designer: the way to a Document Pipeline spec

## Destination

An **implementation-ready spec for the Document Pipeline**: the shared Register (the audit trail), how a Step is defined and declares what it needs, how gaps are filled (wizard first, then chat or a Proposal), and the content of the first four documents: **Requirements spec, Design doc / RFC with ADRs, Threat model (STRIDE), Reliability & DR**. It also covers how documents are stored, versioned, rendered and shown, so building can start. Targets live in [REQUIREMENTS.md](../../REQUIREMENTS.md) (R11); vocabulary in [CONTEXT.md](../../CONTEXT.md).

## Notes

- **Domain:** Designer turns a Rough Draft and Design Brief into a Production Diagram, a Design Rationale, Gap Reports and an arc42 export (built: `internal/report`, `internal/app/document.go`). The Document Pipeline turns the same project into the documents a senior engineer is expected to write, each built on the outputs of the steps before it.
- **Settled when charting** (with vinu, 2026-10-12):
  - First documents: Requirements spec, Design doc / RFC + ADRs, Threat model (STRIDE), Reliability & DR. The existing arc42 export stays and should reuse the pipeline's sources rather than duplicate them.
  - Audit trail: **one shared, append-only Register per Project** of numbered entries: requirements (REQ), assumptions (ASM), constraints (CON), non-goals and out of scope (NG), decisions (DEC) and open questions (Q). Each entry records its source, the Step that introduced it and its status. Every document version cites the entries and input versions it used.
  - Gaps: **wizard first** (a short structured question set per Step, with `unknown` allowed and recorded as an assumption), **then chat** for anything found while writing, and **Proposals** when the gap is in the diagram.
  - Spec before building.
- **Every session:** read `REQUIREMENTS.md`, `CONTEXT.md` and this map first. Use the Skill tool for `grilling` + `domain-modeling` on grilling tickets. Keep `REQUIREMENTS.md` (R11) and `CONTEXT.md` up to date as decisions land.
- **Standing preferences** (from the v1 map): performance and **token efficiency** come first; nothing changes the diagram without an approved Proposal; compliance only reports and suggests; Projects are plain files that diff well in git.
- **Tracker conventions (local markdown):** same as [the v1 map](../map.md). Tickets are `wayfinder/doc-pipeline/tickets/NNN-slug.md` with YAML frontmatter, `status: open|closed`, claim with `assignee:`, `blocked_by: [ids]`. The **frontier** is open, unassigned tickets whose `blocked_by` are all closed. Resolve with a `## Resolution` section, `status: closed` and a line under Decisions so far.

## Decisions so far

<!-- one line per closed ticket, then open the link for the detail the ticket holds -->

- [What is the Register format?](tickets/001-what-is-the-register-format.md): an append-only event log `register/log.jsonl` with a SHA-256 hash chain; 7 entry types (REQ, ASM, CON, NG, DEC, RISK, Q) with type-specific fields; runtime-assigned IDs like `ASM-012`, never reused; source and links on every entry; the Assistant only creates `open` entries, only the user confirms or rejects; text is never edited, only superseded. The Brief stays its own source; Rationale assumptions and trade-offs move into the Register; Gap Reports stay separate. Citing a rejected or superseded entry makes a document stale.
- [Which document templates and standards can we follow?](tickets/008-which-document-templates-can-we-follow.md): Requirements follow the ISO 29148 SRS outline (structure only, paid) with Volere-style fit criteria; Design doc a Google-style outline; ADRs MADR 4.0 (copyable); Threat model the Threat Modeling Manifesto's four questions with STRIDE-per-element (CC BY); Reliability & DR an SLO document, FMEA and NIST SP 800-34 (public domain). Google SRE books are CC BY-NC-ND (concepts only); arc42 is CC BY-SA (our own wording plus credit).
- [What is a Step definition?](tickets/002-what-is-a-step-definition.md): Steps are built-in YAML (inputs, needs, derivers, section schema, entry types they may create); local **derivers** compute facts without tokens; order Brief → requirements → (Conversion) → design-doc, threat-model, reliability → arc42; requirements runs before Conversion when it can but isn't required; each document version records what it was built from and goes stale when any of it changes (never regenerating by itself); one generic `submit_step` tool; upstream documents are passed as summaries.
- [How are gaps found and asked?](tickets/003-how-are-gaps-found-and-asked.md): a token-free pre-check answers needs from Brief, Register, diagram and upstream documents; the rest become a per-Step wizard whose answers are `confirmed` entries and whose `unknown`s become `open` assumptions; few needs block; no live questions while writing, the AI records `Q` entries instead; in chat, `propose_entries` shows a review card before anything is recorded; diagram gaps become linked `RISK` entries with Draft a fix.
- [How are documents stored, versioned and rendered?](tickets/009-how-are-documents-stored-versioned-and-rendered.md): `documents/<step>/vNNN.json` written once, plus a rendered `vNNN.md` for git; versions per Step; arc42 becomes a Step; ADRs are rendered `DEC` entries (MADR 4.0, ADR-0012 = DEC-012); `internal/report` gains Markdown; a document pack zip (documents, ADRs, `register.md`, `log.jsonl`, an index) is the audit hand-off.
- [What goes in the Requirements spec?](tickets/004-what-goes-in-the-requirements-spec.md): ISO 29148 outline in 14 sections; problem statement and user roles block; Brief targets become `confirmed` non-functional `REQ` and `CON` entries; functional requirements drafted from the Rough Draft as `open`; every requirement has priority, rationale and an acceptance criterion; Conversion reads confirmed REQ/CON/NG.
- [What goes in the Design doc and ADRs?](tickets/005-what-goes-in-the-design-doc-and-adrs.md): 12-section design doc built from the Requirements spec, diagram and Register; ADRs are `DEC` entries rendered in MADR 4.0 with status from the entry, superseded rather than edited; Rationale trade-offs are `DEC`s; arc42 and the Design doc render the same entries.
- [How is the Threat model built?](tickets/006-how-is-the-threat-model-built.md): the Manifesto's four questions; the runtime derives the data flow diagram (Kinds → entities, processes, stores; Groups → trust boundaries) and the STRIDE-per-element table; the AI writes threats for applicable cells (boundary-crossing and classified first), rates likelihood × impact, cites Practices and Controls; missing mitigations become `RISK`s; no threat Records in v1.
- [What goes in the Reliability & DR document?](tickets/007-what-goes-in-the-reliability-and-dr-document.md): SLOs per critical journey with computed error budgets, composite availability per journey path (deriver), a failure-mode table seeded from Kinds, backup and recovery against RPO/RTO, a NIST 800-34-style contingency plan; unmet targets become `RISK`s.

## Not yet specified

- **Regenerating only what changed.** Staleness is decided (ticket 002: whole documents, never automatic). Whether a stale document can be regenerated per section to save tokens, and how a regenerated version shows what changed since the last one, is still open.
- **Traceability views.** A requirement → component → Control → document matrix, and a "what changed since the last export" view, built from the Register. Shape depends on the Register format.
- **Audit export.** Exporting the Register (and which document versions cited which entries) as a standalone audit report, possibly alongside the compliance appendix. Depends on the Register format and output formats.
- **Token budget per Step.** How much context each Step sends (Register entries, earlier documents, diagram) and when to summarise earlier documents instead of sending them whole.
- **The later documents** (data inventory and privacy, operational readiness, compliance mapping / SSP, test strategy): in scope for the pipeline's design (the Step format must fit them) but specified after the first four.

## Out of scope

- Infrastructure-as-code or deployment scripts generated from the documents (v1, per `REQUIREMENTS.md`).
- Review and sign-off workflows between several people (Designer is solo-first in v1).
- Editing documents as free text inside Designer: documents are generated from the Register, answers and diagram; changes go in through those.
