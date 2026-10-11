---
id: "006"
title: "How is the Threat model built?"
labels: [wayfinder:grilling]
status: closed
assignee: vinu (with claude)
blocked_by: ["001", "002"]
parent: map
---

# How is the Threat model built?

## Question

A STRIDE threat model per element and data flow, using what the diagram already knows. How is it built, and how much can the runtime do without the AI?

- **Data flow diagram:** Nodes as processes and data stores, Edges as flows, Groups (boundaries, subnets, networks, accounts) as trust boundaries, Data Classification as assets. Which flows cross a boundary, computed by the runtime.
- **Enumeration:** STRIDE per element type (the standard element/threat table) computed locally, then the AI judges which apply, their likelihood and impact, and the existing mitigation. Rating scheme (e.g. high/medium/low, or a simple DREAD-like score)?
- **Mitigations:** linked to Knowledge Base Practices and Controls where they exist (and to Gap Report items), so a mitigation that's missing on the diagram becomes a Proposal or a risk.
- **User input:** threat actors and assets they care most about (wizard).
- **Knowledge Base:** do we need threat Records (licence check: OWASP and Microsoft threat modeling material), or are Practices enough?
- **Register:** accepted risks become DEC or ASM entries with their owner.

## Resolution

Decided 2026-10-12, taking the recommended option throughout (vinu: "find the recommended solution and proceed with it"). Structures follow [research 008](008-which-document-templates-can-we-follow.md), in our own wording.

**Structure:** the Threat Modeling Manifesto's four questions (CC BY 4.0: adapt with credit), with a STRIDE-per-element threat table.

| # | Section | How it's built |
|---|---|---|
| 1 | What are we working on? | Scope (from the Requirements spec), assets (Data Classification) and a **data flow diagram derived by the runtime** from the Production Diagram |
| 2 | What can go wrong? | A **STRIDE-per-element table computed by the runtime**; the AI fills in concrete threats only where they apply |
| 3 | What are we going to do about it? | A response per threat: mitigate (with the mitigation), eliminate, transfer or accept |
| 4 | Did we do a good job? | Coverage numbers (elements and flows reviewed, threats per response, open risks) and a short review checklist |

**Deriver: the data flow diagram** (no tokens):
- **External entities:** Nodes of `external.*` Kinds.
- **Processes:** compute, network and security Nodes that handle requests.
- **Data stores:** `data.*` Nodes, plus log, audit and backup stores; message queues and streams count as data stores.
- **Data flows:** Edges.
- **Trust boundaries:** Groups of kind boundary, account, network and subnet.

The runtime marks every flow that **crosses a boundary**, and every store or flow carrying PHI, PII or confidential data.

**Deriver: the STRIDE-per-element table:**
- external entity: spoofing, repudiation;
- process: all six;
- data store: tampering, information disclosure, denial of service, plus repudiation for log stores;
- data flow: tampering, information disclosure, denial of service.

Research 008 marks the data-store and data-flow rows as resting on secondary sources: verify against Shostack's book before release.

**What the AI does:** for each applicable cell it writes a concrete threat, or marks the cell not applicable with a reason. Boundary-crossing flows and classified stores come first, so a large diagram stays within budget: low-risk cells are grouped.
- **Rating:** likelihood and impact (low, medium, high) giving a risk level from a 3×3 matrix.
- **Mitigations** cite Knowledge Base Practices and Controls, and diagram elements that already provide them (e.g. the WAF).
- **A missing mitigation** becomes a `RISK` (open) linked to the elements, with **Draft a fix** (ticket 003).
- **An accepted threat** is a `RISK` plus an accepting `DEC`.

**Knowledge Base:** no threat Records in v1; Practices and Controls cover the mitigations. Threat Records can come later, under the licence rules (Microsoft and OWASP material is structure and facts only).

**Wizard questions:** threat actors of concern (external attacker, malicious insider, compromised third party, compromised account; multi-select), the assets that matter most (preselected from Data Classification), and security controls that exist but aren't on the diagram (each becomes a `CON`).
