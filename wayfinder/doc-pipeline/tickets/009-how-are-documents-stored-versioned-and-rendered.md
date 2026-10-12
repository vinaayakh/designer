---
id: "009"
title: "How are documents stored, versioned and rendered?"
labels: [wayfinder:grilling]
status: closed
assignee: vinu (with claude)
blocked_by: ["002"]
parent: map
---

# How are documents stored, versioned and rendered?

## Question

How do pipeline documents live in the Project folder and leave Designer?

- **Source of truth:** structured JSON per document version (like `documents/vNNN-arc42.json`) rendered on demand, or Markdown files that are themselves the document (diff and review well in git)? Both?
- **Versioning:** a new document version per generation, never edited (like Gap Reports), each recording the Step definition version, input versions and cited Register entries.
- **Rendering:** reuse `internal/report` (Word, PDF via print, add Markdown), one document per file or one bundle; ADRs as one file each.
- **Export:** a "document pack" (all current documents plus the Register) for handing to reviewers or auditors.
- **arc42:** does the existing export become a pipeline Step, reading the same sources?

## Resolution

Decided 2026-10-12, taking the recommended option throughout (vinu: "find the recommended solution and proceed with it").

**Storage: structured source plus a Markdown copy.**
- **Source of truth:** `documents/<step>/vNNN.json`, the submitted sections, the build record from ticket 002 (inputs, cited entries, model, time), and the Step definition version. Written once, never edited, like Gap Reports.
- **Markdown copy:** `documents/<step>/vNNN.md` is rendered when the version is created, so documents can be read, reviewed and diffed in git without Designer. It's derived and can be regenerated.

**Versions** are numbered per Step (Requirements v3 is independent of Production v6), and each records which Production version it used.

**arc42 becomes a Step.** Its existing text (`documents/vNNN-arc42.json`) moves to `documents/arc42/` on first load. It reads the same Register and upstream documents, so a fact lives once: decisions are `DEC`, risks are `RISK`, assumptions are `ASM`.

**ADRs** are rendered from `DEC` entries, not stored separately. ADR number = DEC number (`DEC-012` → `ADR-0012`), in MADR 4.0 format. A superseded decision renders as a superseded ADR pointing to its replacement. They're written out as one file each on export.

**Rendering:** `internal/report` gains a Markdown renderer next to Word and HTML/PDF. Section kinds map to its existing blocks (paragraphs, tables, lists, callouts, the diagram figure). `entries` sections are tables of Register entries with ID, status and source.

**Export: a document pack.** A zip containing:
- every current document in the chosen format, plus Markdown;
- the ADRs;
- `register.md` (every entry with status, source and history) and the raw `log.jsonl`;
- an index listing each document's version, what it was built from, and whether it's stale.

That pack is the audit hand-off.

## Implementation notes (2026-10-12)

Built as decided, with two deviations, both taking the lower-risk option:

- **arc42 keeps its text per Production version** (`documents/vNNN-arc42.json`, not moved to `documents/arc42/`). Its text describes one diagram version, so it stays keyed to it. It is a Step in the Documents tab (status: written for vN / out of date, Export Word / PDF), and its requirements, constraints, assumptions, non-goals, decisions (ADR-NNNN = DEC-NNN), risks and open questions now come from the Register; the Design Rationale is only the fallback for a Project without one. Its writing task also sees the confirmed requirements. The arc42 credit line (CC BY-SA 4.0) is in Appendix B. The open question from research 008 (whether reusing arc42's whole heading list needs a legal opinion before selling) still stands.
- **`internal/report` has no general Markdown renderer.** Pipeline documents already have a Markdown copy (`vNNN.md`). The document pack holds those, plus ADRs, `register.md`, `register/log.jsonl` and a README index. arc42 stays Word / PDF, since it needs the diagram image from the browser.
