---
id: "009"
title: "How do Scenario changes get back into the diagram?"
labels: [wayfinder:grilling]
status: closed
assignee: claude (recommended options, for vinu's review)
blocked_by: ["006"]
parent: map
---

# How do Scenario changes get back into the diagram?

## Question

Settled when charting: **Apply** shows the Scenario's changes in the Proposal review overlay and saves one new Production version. Decide how.

- **Which changes apply:** structural changes and sizing go into the diagram. Traffic, routes, events and the seed stay in the Scenario. Does Edge behaviour (timeouts, retries) go into the diagram?
- **Review:** reuse the Proposal overlay as is (step through, accept or skip each change, one Accept for all), with the user as the author instead of the Assistant. Do partial accepts work the same way, with dependencies resolved by the runtime?
- **Base moved on:** what happens when the Production Diagram has a newer version than the Scenario's base?
- **Afterwards:** is the Scenario rebased onto the new version automatically? Does the Design Rationale need updating, and by whom (the Assistant only if asked)?
- **Register:** does applying record a `DEC` entry ("Added a read replica: the database saturated at 3k rps in Scenario Black Friday")? Proposed by the runtime as `open`, confirmed by the user, as the Register rules require.

## Resolution

Decided 2026-10-15, taking the recommended option throughout.

**What applies:**
- **Applies to the diagram:** structural changes, Sizing (`set_sizing`) and Edge behaviour (`set_flow`). Timeouts, retries and breakers are design decisions, so they belong in the diagram.
- **Stays in the Scenario:** traffic, Routes, events, seed and Profile mode.

**Review: the Proposal overlay, unchanged.**
- Apply builds a Proposal from the Scenario's diagram changes. They already use the Proposal operations (ticket 006), so this is a direct translation.
- Its author is shown as "You, from Scenario Black Friday".
- Stepping through, partial accepts with dependencies resolved by the runtime, one version, one Undo: all as for Proposals (ticket 010 of the v1 map).
- This keeps a single path into the diagram. Nothing reaches it without the user accepting a review.

**Base moved on:** Apply is disabled with "Rebase first" until the Scenario is rebased onto the current version (ticket 006), so the review always shows changes against what's really there.

**Afterwards.**
- The Scenario is rebased onto the new version, and the accepted changes drop out of its change list. Skipped ones stay, so the user can keep simulating with them.
- The Design Rationale is not changed automatically. The Assistant updates it only when asked, as with any edit made by hand.

**Register.**
- After Apply, the runtime offers one `DEC` entry per accepted group of changes, written from the run's numbers. For example: "Added a read replica to Orders DB. In Scenario *Black Friday*, Orders DB saturated at ≈3.2k rps (p99 1.4 s against 300 ms). With the replica, the breaking point moved to ≈6.1k rps."
- The entries are `open` and cite the Scenario and its run, and the user confirms or rejects them in the Register tab (Register rules: the runtime and the Assistant only propose).
- The user can dismiss the offer.
