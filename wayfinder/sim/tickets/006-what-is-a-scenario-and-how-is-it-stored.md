---
id: "006"
title: "What is a Scenario and how is it stored?"
labels: [wayfinder:grilling]
status: closed
assignee: claude (recommended options, for vinu's review)
blocked_by: ["003", "005"]
parent: map
---

# What is a Scenario and how is it stored?

## Question

The user plays in a Scenario, a working copy of one Production Diagram version (settled when charting). Pin down what it contains and how it's kept.

- **Contents:** the base Production version, the changes made to it (sizing, Edge behaviour, added or removed Nodes and Edges), traffic routes and shape, scripted events ("at 2:00, lose zone a"), the Profile choice (typical, pessimistic…), the seed.
- **Changes as a diff or a copy?** A list of changes against the base version stays small and can be shown for review (ticket 009). A full copy of the scene is simpler but large.
- **Base moves on:** when a newer Production version exists, can the Scenario be rebased onto it, or is it marked stale like a document?
- **Several Scenarios:** "Normal day", "Black Friday", "Lose a region". Saved, named, compared.
- **Runs:** is a run (the metrics it produced) saved? Where, and for how long?
- **Files:** what goes in the Project folder (plain, diff-friendly) versus `.designer/` (runtime state).

## Resolution

Decided 2026-10-15, taking the recommended option throughout.

**A Scenario is a change list, not a copy.** It's stored as `sim/scenarios/<slug>.yaml` in the Project folder: plain, small and diff-friendly.

```yaml
name: Black Friday
base: 7                      # Production Diagram version
seed: 1234
profile_mode: typical        # typical | optimistic | pessimistic
traffic: { base_rps: 5000, shape: spike, factor: 4, at: "10:00", minutes: 30 }
routes:
  - { name: GET /product, entry: kq3Xz, share: 0.8 }
  - { name: POST /checkout, entry: kq3Xz, share: 0.2, latency_ms: 500, mult: { "a8Fj2": 3 } }
events:
  - { at: "20:00", type: zone-down, target: g4 }
changes:                     # the same operations as a Proposal, against element IDs; +new for added items
  - { op: set_sizing, node: n1Ab, sizing: { count: 6 } }
  - { op: add_node, ref: +cache, kind: data.cache, service: aws.elasticache-redis, label: Product cache }
  - { op: insert_on_edge, edge: e9Qw, node: +cache }
```

- **Changes use the Proposal operation vocabulary** (ticket 010 of the v1 map), plus `set_sizing` and `set_flow`. That makes Apply (ticket 009) a direct translation, and the same validation runs on both.
- **The working scene is built when Sim Mode opens:** the base version plus the changes, in memory, never written. A 500-Node scene builds in milliseconds.
- **Base moves on.** When a newer Production version exists, the Scenario shows "built on v7; v9 is current" with **Rebase**. Rebase replays the changes onto the current version. A change whose target was removed or already matches is dropped and listed, so the user sees what fell out. Nothing rebases by itself.
- **Several Scenarios** per Production Diagram, named by the user. The first time Sim Mode opens, a **"Normal day"** Scenario is created from the Brief (steady traffic, default Routes), and it's only written once the user changes something.

**Runs.**
- A run isn't part of the design, so it isn't saved in the Project folder.
- The latest run of each Scenario goes in `.designer/sim/<slug>/`: the run summary (ticket 010), the dashboard series at their coarsest retention tier, and the edit log with tick numbers. A reload shows the last dashboard, and the run can be replayed exactly (ticket 002).
- Comparing runs works on the current session's runs plus the saved latest one (ticket 007).
