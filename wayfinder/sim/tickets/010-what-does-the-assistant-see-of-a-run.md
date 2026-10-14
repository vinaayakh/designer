---
id: "010"
title: "What does the Assistant see of a run?"
labels: [wayfinder:grilling]
status: closed
assignee: claude (recommended options, for vinu's review)
blocked_by: ["007", "009"]
parent: map
---

# What does the Assistant see of a run?

## Question

The simulation never calls the Assistant. But the user may ask "why is checkout slow?" or "how do I survive 3× traffic for less?". What does the Assistant get, and what can it do?

- **Run summary:** a compact, token-cheap text: the Scenario, traffic, which targets broke and when, bottleneck order with utilisation, the top routes' p95/p99 against target, cost, and events. A few hundred tokens at most, never raw series.
- **Sizing in the compact format:** from ticket 003.
- **Tools:** does the Assistant need a tool to run a Scenario itself ("try 2 replicas and tell me") or only to read the summary? Runs are free, but each round trip costs tokens.
- **Suggestions:** a Proposal against the Scenario (not the diagram), so the user can try it in the sim before applying. Does `submit_proposal` need a target?
- **Knowledge Base:** do Practices (cache-aside, read replicas, queue-based load levelling) get linked from the summary, so the Assistant cites them?

## Resolution

Decided 2026-10-15, taking the recommended option throughout.

**Run summary, computed in the browser.**
- The worker builds a compact text summary of a run when the run pauses, a goal breaks, or the user asks.
- The page posts it to the runtime, which stores it with the run in `.designer/sim/<slug>/` (ticket 006).
- It is capped at about 300 tokens, uses the session's Aliases, and never includes raw series:

```
scenario "Black Friday" on v7 · typical · seed 1234
traffic spike 5k→20k rps at 10:00 · goal p99 300ms, err<1%
broke 10:42 at ≈14.1k rps: p99 1.4s, err 3.2%
saturation: n2 db 100% (10:38) · n1 api 92% (×10 max) · n5 cache 41%
routes: GET /product p99 410ms (target 300) · POST /checkout p99 2.1s err 9% (target 500)
queues: q1 backlog 3m40s growing
cost ≈$14.20/h (prices 2026-10, us-east-1)
edits: 10:20 n1 ×4→×10 · 10:31 +cache n5 on n1→n2
practices: cache-aside, read-replicas
```

**When the Assistant sees it.** Only on request, to save tokens:
- the user adds the run to the Assistant's context (an "Add run" chip, like adding Nodes);
- the user clicks "Ask the Assistant" on a breaking-point summary (ticket 008).

**Tools.**
- One new read tool, `get_sim_summary(scenario)`, returns the latest stored summary.
- There's no tool that runs the simulation. The user plays and the Assistant advises. A tool that runs the sim would invite many token-costly round trips for something the user can do for free in a second.

**Suggestions target the Scenario.**
- `submit_proposal` gains an optional `target: scenario:<slug>`. Such a Proposal is reviewed in the same overlay, and accepting it adds the changes to the Scenario, not the diagram.
- The user then plays it, and applies it to the diagram later through ticket 009.
- The Assistant may also propose Sizing (`set_sizing`) and Edge behaviour (`set_flow`).

**Knowledge Base.** A small built-in table maps bottleneck patterns to Practice IDs:
- a saturated read-mostly relational store → cache-aside, read replicas;
- a growing queue backlog → queue-based load levelling, consumer scaling;
- a retry storm → circuit breaker, backoff with jitter;
- one hot partition → partition key design.

The summary lists the matched IDs, and the Assistant fetches them with `get_record`, so its advice is grounded and cited. Practices missing from the Knowledge Base are listed as new pilot Records to write, not invented on the fly.
