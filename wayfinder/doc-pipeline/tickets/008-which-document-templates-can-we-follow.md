---
id: "008"
title: "Which document templates and standards can we follow?"
labels: [wayfinder:research]
status: closed
assignee: claude (research)
blocked_by: []
parent: map
---

# Which document templates and standards can we follow?

## Question

For each of the first four documents, which widely used template or standard should its structure follow, and what are we allowed to reuse under our licensing rules (ADR 0002: public domain verbatim, CC BY adapted with credit, CC BY-SA and restricted sources as structure and facts in our own words)?

- **Requirements:** ISO/IEC/IEEE 29148 (paid; structure only?), Volere, common SRS and PRD templates.
- **Design doc and ADRs:** common design doc / RFC templates (e.g. Google-style design docs), MADR, Michael Nygard's ADR format, and their licences.
- **Threat model:** Microsoft STRIDE and its element/threat table, OWASP Threat Modeling (Cheat Sheet, Threat Modeling Manifesto), NIST SP 800-154 (draft); licences.
- **Reliability & DR:** the Google SRE books' SLO guidance and templates, NIST SP 800-34 (contingency planning, public domain), FMEA basics.
- **arc42:** confirm its licence (CC BY-SA 4.0) and what that means for our generated documents.

Output: one recommended structure per document with sources, licence tier and what we may copy versus describe.

## Resolution

Researched 2026-10-12. Findings with sources and licence checks: `wayfinder/doc-pipeline/research/008-document-templates.md` on branch `research/008-document-templates` (commit f548ae7).

| Document | Structure to follow | What we may reuse (ADR 0002) |
|---|---|---|
| Requirements spec | ISO/IEC/IEEE 29148:2018 SRS outline (purpose, scope, glossary, overall description, functional, interfaces, quality, data, verification, open issues, traceability), plus Volere's rationale and fit criterion per requirement | 29148 is paid: structure and facts in our own words. Volere forbids commercial use: link only |
| Design doc / RFC | Context and scope, goals and non-goals, design, cross-cutting concerns, alternatives, drawbacks, rollout, open questions | Google-style post has no licence: structure only. Rust RFC prompts are MIT/Apache-2.0 (formal status unconfirmed) |
| ADRs | MADR 4.0, with Nygard's five sections as the short form | Both CC0 / MIT: may be copied |
| Threat model | Threat Modeling Manifesto's four questions; a data flow diagram from the architecture diagram; STRIDE-per-element (external entity S R; process all six; data store T I D, plus R for logs; data flow T I D); responses (mitigate, eliminate, transfer, accept) | Manifesto CC BY 4.0: adapt with credit. OWASP cheat sheet CC BY-SA and Microsoft: structure only. NIST SP 800-154 (draft) public domain |
| Reliability & DR | SLO document (SLIs and SLOs, rationale, error budget and policy), FMEA table, recovery objectives (MTD, RTO, RPO) and backup, NIST SP 800-34 contingency plan phases | NIST 800-34 public domain: verbatim with credit. Google SRE books are CC BY-NC-ND: concepts and structure only |
| arc42 | Existing export | CC BY-SA 4.0 covers its headings and text: our own wording plus a credit line |

Unverified (see the research file): exact 29148 clause 9.6 headings (paid, rebuilt from secondary sources); whether copying a whole heading list is allowed (needs a legal opinion before selling); STRIDE data-store and data-flow rows rest on secondary sources; the Rust RFC template's licence; FMEA standards' licences.

Follow-up: the existing arc42 export copies arc42's section names; check it against this before release.
