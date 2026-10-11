---
id: "004"
title: "What goes in the Requirements spec?"
labels: [wayfinder:grilling]
status: closed
assignee: vinu (with claude)
blocked_by: ["001", "002"]
parent: map
---

# What goes in the Requirements spec?

## Question

The Requirements spec is the root document most others build on. What are its sections, where does each come from, and what must the user supply?

- **Sections:** purpose and scope, users and roles, use cases or user journeys, functional requirements, non-functional requirements (from the Brief), constraints, assumptions, non-goals and out of scope, open questions. Which standard to follow in structure (lightweight IEEE 29148 / a typical SRS / a PRD-style spec), see the template research ticket.
- **Deriving from the Rough Draft:** functional requirements from flows (Edges and their labels), Notes and Node labels; how confident is that, and how is it shown (each REQ marked "derived from the diagram, confirm")?
- **What only the user knows:** problem statement, user roles, key use cases, success metrics, explicit non-goals: the wizard question set.
- **Requirement quality:** IDs, priority (MoSCoW?), and a testable acceptance criterion per requirement.
- **Timing:** before Conversion (so Design reads confirmed requirements) or after; how it affects the existing Conversion task.

## Resolution

Decided 2026-10-12, taking the recommended option throughout (vinu: "find the recommended solution and proceed with it"). Structures follow [research 008](008-which-document-templates-can-we-follow.md), in our own wording.

**Structure:** the ISO/IEC/IEEE 29148 software requirements outline (structure only; the standard is paid), with a rationale and an acceptance criterion on every requirement (the idea Volere calls a fit criterion; Volere itself is link-only).

| # | Section | Comes from |
|---|---|---|
| 1 | Document control | Build record: version, inputs, Register entries cited, open-assumption count |
| 2 | Purpose and problem statement | Wizard (blocking): the problem and who has it → `REQ` kind `goal` |
| 3 | Scope | In scope: the Rough Draft's components; out of scope and non-goals: `NG` entries (wizard asks for explicit non-goals) |
| 4 | Users and roles | Wizard (blocking: at least one role); external actors on the diagram are suggested |
| 5 | Operating environment and constraints | Brief (cloud, regions, residency, compliance, budget, team) → `CON` entries created by the runtime, `confirmed`, linked to their Brief field |
| 6 | Assumptions | `ASM` entries (Brief `unknown` answers, wizard `unknown`, the AI's own) |
| 7 | Use cases | Wizard asks for the top 3–7 user journeys (optional); otherwise the AI drafts them from the Rough Draft's flows as `open` |
| 8 | Functional requirements | `REQ` kind `functional`, grouped by use case. The AI derives them from Edges and their labels, Notes and Node labels (`source: diagram`, `open`, marked "derived from the diagram — confirm") plus wizard answers |
| 9 | External interfaces | External actors and systems on the diagram (deriver), with their flows |
| 10 | Quality requirements | `REQ` kind `non-functional` from the Brief's targets (latency, throughput, availability, RPO, RTO, data volume), written by the runtime from a template sentence, `confirmed` (the user gave the number) |
| 11 | Data requirements | Data Classification, retention (wizard, optional), residency |
| 12 | Verification | For each `REQ`: test, inspection, analysis or demonstration (AI proposes) |
| 13 | Open questions | `Q` entries |
| 14 | Traceability | `REQ` → diagram elements (links); later Steps add their columns (design sections, threats, SLOs) |

**Every requirement** has an ID, kind, MoSCoW priority (`must`, `should`, `could`), statement, rationale, testable acceptance criterion and source. The Step rejects a submission with a requirement missing any of these.

**Wizard questions:** problem statement (blocking), user roles (blocking), top user journeys, success metrics, explicit non-goals, data retention, external systems not drawn. Everything except the two blocking ones accepts `unknown`.

**Timing:** after the Brief, before Conversion when possible (ticket 002). Conversion's Design message then includes the confirmed `REQ`, `CON` and `NG` entries in compact form, and its assumptions cite them.
