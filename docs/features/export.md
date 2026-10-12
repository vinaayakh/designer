# Export

From **Export** in the header:

- **PNG image**, **SVG image**: the current view.
- **arc42 document · Word / PDF**: the architecture document for the Production version you're viewing. It's built from the Brief, the diagram, the Gap Report and the Register (requirements, constraints, assumptions, decisions as ADRs, risks, open questions). The Assistant can write the parts stored data can't (component responsibilities, runtime scenarios, crosscutting concepts, quality scenarios) once per version, or you can export without them. PDF uses the browser's Save as PDF. The same export is the last Step in the Documents tab.

From the Documents tab:

- **Download document pack**: a zip for reviewers or auditors. It holds each document's latest version as written (Markdown), an ADR file per decision in MADR 4.0 (`DEC-012` is `ADR-0012`; superseded ones point to their replacement), `register.md` with every entry's source and history, the raw hash-chained `register/log.jsonl`, and a README index giving each document's version, what it was built from, and whether it's stale.
