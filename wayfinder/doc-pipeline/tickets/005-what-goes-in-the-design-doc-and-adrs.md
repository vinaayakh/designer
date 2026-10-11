---
id: "005"
title: "What goes in the Design doc and ADRs?"
labels: [wayfinder:grilling]
status: closed
assignee: vinu (with claude)
blocked_by: ["001", "002"]
parent: map
---

# What goes in the Design doc and ADRs?

## Question

The Design doc (RFC) explains the Production design to reviewers; ADRs record each significant decision on its own. What are their contents and how do they relate to what exists?

- **Design doc sections:** context, goals and non-goals, the proposed design (diagram, components, data flows), alternatives considered, cross-cutting concerns (security, reliability, cost, observability), risks, rollout, open questions. Which template to follow (see the template research ticket).
- **ADRs:** format (MADR or Nygard), one file per decision, numbering, status (proposed, accepted, superseded), and how they map to Design Rationale trade-offs and DEC Register entries (is an ADR just a rendered DEC entry?).
- **Alternatives:** the Rationale has `over` per trade-off; what extra the user is asked (alternatives they rejected and why).
- **Overlap with arc42:** the arc42 export already covers decisions, building blocks and risks. Which sources do both share, so a fact lives once?
- **Change over time:** a Proposal that changes a decision supersedes its ADR rather than editing it.

## Resolution

Decided 2026-10-12, taking the recommended option throughout (vinu: "find the recommended solution and proceed with it"). Structures follow [research 008](008-which-document-templates-can-we-follow.md), in our own wording.

**Design doc structure** (the common design doc / RFC outline; structure only, our wording):

| # | Section | Comes from |
|---|---|---|
| 1 | Document control | Build record |
| 2 | Context and scope | Requirements spec summary (goals, scope) and the Brief; not asked again |
| 3 | Goals and non-goals | `REQ` kind `goal` and `must` requirements; `NG` entries |
| 4 | Design overview | The Production Diagram (figure), the Rationale summary, the main request path |
| 5 | Detailed design | Per Group (boundary, network, tier): its components, their responsibilities, interfaces and data flows (the deriver that builds arc42's building blocks) |
| 6 | Data | Data stores, Data Classification, flows of classified data |
| 7 | Cross-cutting concerns | Security (summary and link to the Threat model), reliability (link to Reliability & DR), observability, compliance (latest Gap Report), cost (qualitative, from the Brief's budget) |
| 8 | Alternatives considered | Each `DEC`'s `over` and `because`, plus alternatives the user rejected (wizard, optional) |
| 9 | Risks and drawbacks | `RISK` entries |
| 10 | Rollout and migration | Wizard: greenfield or replacing a system, phases, cut-over constraints (optional) |
| 11 | Open questions | `Q` entries |
| 12 | Decision log | Every `DEC`, linking its ADR |

**ADRs are rendered `DEC` entries** in MADR 4.0 (which may be copied): title, status, context, decision drivers, considered options, decision outcome and consequences. `ADR-0012` is `DEC-012`. Its status comes from the entry (`open` → proposed, `confirmed` → accepted, `superseded` → superseded by the new ADR, `rejected` → rejected). Nygard's five-section form is offered as the short format. A Proposal that changes a decision creates a new `DEC` that supersedes the old one; ADRs are never edited.

**The Design Rationale's trade-offs become `DEC` entries** (ticket 001), so the Design doc, the ADRs and arc42 section 9 all render the same entries.

**Overlap with arc42:** arc42 becomes a Step that renders the same sources (ticket 009). The Design doc is the reviewer's narrative ("why this design"); arc42 is the reference structure ("what is where"). Neither stores its own copy of a decision, risk or assumption.

**Wizard questions:** rollout context, rejected alternatives, reviewers and approvers (names only, for document control; no sign-off workflow in v1).
