---
id: "008"
title: "What does Sim Mode look like?"
labels: [wayfinder:grilling, wayfinder:prototype]
status: closed
assignee: claude (recommended options, for vinu's review)
blocked_by: ["001", "006", "007"]
parent: map
---

# What does Sim Mode look like?

## Question

Sim Mode should feel like a simulation game while staying a design tool. Settle the screen and the interactions, with a clickable prototype.

- **Entering and leaving:** from a Production Diagram (a Simulate button, or a tab beside the diagram views?). Pick or create a Scenario. The canvas is locked for normal editing while in Sim Mode, as it is while a Proposal waits.
- **Canvas overlay:** dots moving along Edges with density from flow rate, Node heat from green to red by utilisation, badges for queue depth and errors, instance pips (×3, one warming up), failed Nodes greyed out.
- **Controls:** play/pause, speed (1×, 10×, 100×), traffic slider and shape picker, events menu (kill Node, lose zone, slow dependency), Find breaking point.
- **Inspector:** select a Node or Edge to see its live numbers and change its sizing (count, size, instance type or Service, scaling rule, pool size) and Edge behaviour (timeout, retries, cache hit). Changes apply live.
- **Structural edits:** a small palette (cache, queue, read replica, load balancer, CDN). Dropping one onto a Node or Edge inserts it in the right place, placed locally without ELK, as Proposals are.
- **Goals:** SLO and budget pass or fail always visible; a short "what broke first" summary when a target breaks.
- **Dashboard placement:** from ticket 007.
- **Leaving:** keep the Scenario, discard it, or Apply (ticket 009).

## Resolution

Decided 2026-10-15, taking the recommended option throughout. The spike page doubles as the prototype: `spikes/sim-001/` (control bar, read-only canvas with overlay, inspector, docked dashboard). Screenshots are in `spikes/sim-001/results/`.

**Entering.**
- A **Simulate** button on the Production Diagram toolbar opens Sim Mode in place, on the same canvas and viewport.
- A Scenario picker in the control bar lists the diagram's Scenarios plus "New Scenario". The first visit opens "Normal day" (ticket 006).
- **The canvas is read-only in Sim Mode**, as it is while a Proposal waits. Selection uses our own hit-testing, as on the frozen Rough Draft (ticket 015 of the v1 map), and every change goes through the inspector and palette into the Scenario.

**Canvas overlay** (Canvas 2D, ticket 001):
- **Heat** is the Node's outline colour (green below 60% utilisation, amber to 85%, red above) plus a thin utilisation bar under the card. There's no fill, because a fill blurs the label.
- **Badges:** `×3 +1` (ready and warming instances), utilisation, and errors when above 0.5%. Queues show backlog age.
- **Failed Nodes** are greyed out. An open breaker shows as a broken Edge.
- **Flow:** dots move along Edges, their count log-scaled from flow and capped at 12 per Edge. Red dots show the share of calls failing.
- **Level of detail:** below 30% zoom, Edges are drawn with thickness by flow instead of dots, and badges are hidden.
- The overlay follows `snapshotPan`'s in-flight view during gestures.

**Control bar:**
- Scenario picker; play/pause (Space); speed 1×, 10×, 100×.
- Traffic slider (log scale) with a shape picker; events menu (kill Node, lose zone, slow dependency, restore).
- Find breaking point.
- The goal pill (SLO met / broken, with hysteresis) and budget against the Brief.
- Simulated clock, marked "≈ simulated"; **Apply to diagram…**; **Exit**.

**Inspector** (a Sim tab in the right sidebar, next to the Assistant):
- For the selected Node: live numbers, then the Sizing fields that apply to its Kind (ticket 003). The size dropdown comes from the Profile's sizes and shows price. Each Profile number shows its range and confidence.
- For an Edge: `flow` fields (ticket 005).
- Kill / Restore.
- Changes apply live at the next tick and add an event marker.

**Structural palette:** cache, queue, read replica, load balancer, CDN, circuit breaker. Each one is dragged onto a Node or Edge, with clear rules:
- **Cache onto an Edge into a store:** caller → cache → store, cache-aside with a default hit ratio of 0.9.
- **Queue onto a sync Edge:** caller → queue → callee. The call becomes async, and the callee becomes a consumer.
- **Read replica onto a relational store:** the read share of its incoming Edges is split across the replicas.
- **Load balancer onto a Node:** it goes in front of the Node, and the Node's callers are rewired to it.
- **CDN onto an entry Node:** it goes in front, with a default hit ratio of 0.7.
- **Circuit breaker onto an Edge:** turns `breaker` on.

New Nodes are placed locally next to their target without ELK, as Proposals are (ticket 016 of the v1 map).

**Goals and breaking.**
- When a goal breaks, a short summary appears over the canvas: what broke, at what traffic, and the saturation order ("api saturated at ≈670 rps; db at 50%").
- It offers **Show on dashboard** and, if the Assistant is enabled, **Ask the Assistant** (ticket 010).

**Leaving.**
- Exit keeps the Scenario's changes: they're saved as the user makes them.
- Discard changes resets the Scenario to its base.
- Apply to diagram opens the review (ticket 009).
