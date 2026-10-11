---
id: "007"
title: "What goes in the Reliability & DR document?"
labels: [wayfinder:grilling]
status: closed
assignee: vinu (with claude)
blocked_by: ["001", "002"]
parent: map
---

# What goes in the Reliability & DR document?

## Question

The Brief already has availability, latency, RPO and RTO targets, and the diagram shows replicas, zones, backups and queues. What does the Reliability & DR document contain, and what is computed versus written?

- **SLOs and SLIs:** one per user-facing journey (from the Requirements spec) with targets from the Brief; error budgets.
- **Failure modes:** per component (a light FMEA): what fails, the effect, detection, mitigation already on the diagram, and gaps. How much comes from Component Kind (e.g. single-zone data store) computed locally?
- **Dependencies:** external services and their SLAs (user input), and whether the composite availability can meet the target (a simple calculation the runtime can do?).
- **Backup and restore, DR:** how each data store meets RPO and RTO, restore testing, region strategy.
- **Gaps:** a target the design can't meet becomes a Proposal or an accepted risk in the Register.

## Resolution

Decided 2026-10-12, taking the recommended option throughout (vinu: "find the recommended solution and proceed with it"). Structures follow [research 008](008-which-document-templates-can-we-follow.md), in our own wording.

**Structure:** an SLO document (Google SRE concepts; the books are CC BY-NC-ND, so concepts and structure only), a failure-mode table (FMEA, a generic method), and a contingency plan in NIST SP 800-34's phases (public domain).

| # | Section | How it's built |
|---|---|---|
| 1 | Service and critical user journeys | Use cases from the Requirements spec, marked critical by the user (wizard) |
| 2 | SLIs and SLOs | One row per critical journey: SLI definition, target (from the Brief's availability and latency), window (28 days by default) |
| 3 | Error budget and policy | Budget computed by the runtime (e.g. 99.9% over 28 days = 40 min 19 s); the policy (what happens when it's spent) drafted by the AI, `ASM` until confirmed |
| 4 | Dependencies and composite availability | **Deriver:** for each critical journey's path through the diagram, combines component availabilities in series and replicas in parallel. Component figures come from the wizard (or `unknown` → an `ASM` per component); provider SLAs are not shipped. It flags a path that can't reach the target |
| 5 | Failure modes | **Deriver:** one row per component, with failure modes suggested from its Kind and attributes (single instance, single zone, no backup, no queue between services). The AI writes effect, detection (linked observability components) and mitigation, and rates severity, likelihood and detectability 1–5 (risk priority number) |
| 6 | Backup and recovery | Per data store: backup method, RPO and RTO compared with the Brief, restore testing. Maximum tolerable downtime from the wizard (or an `ASM`) |
| 7 | Contingency plan | Activation and notification, recovery, reconstitution; testing and maintenance (NIST SP 800-34 phases) |
| 8 | Gaps | Every target the design can't meet becomes a `RISK` with **Draft a fix**, or an accepted `RISK` plus `DEC` |

**Wizard questions:** which journeys are critical, maximum tolerable downtime, component or dependency availability figures, on-call and escalation (optional), restore-test cadence (optional). All accept `unknown`.

**What's computed versus written:** error budgets, composite availability and the failure-mode skeleton are computed locally. The AI writes effects, mitigations, the error budget policy and the contingency procedures, and every number it uses traces to the Brief, a wizard answer or an `ASM`.
