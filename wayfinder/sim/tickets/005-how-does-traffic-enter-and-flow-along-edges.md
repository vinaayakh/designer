---
id: "005"
title: "How does traffic enter and flow along Edges?"
labels: [wayfinder:grilling]
status: closed
assignee: claude (recommended options, for vinu's review)
blocked_by: ["002"]
parent: map
---

# How does traffic enter and flow along Edges?

## Question

Where does traffic come from, and how does one request at the front become work at each Node behind it?

- **Entry points and routes:** which Nodes receive outside traffic (users, external systems)? Is a route (e.g. `GET /product`, `POST /checkout`) a named path through the graph with its own share of traffic, its own SLO and its own latency on the dashboard? Or is everything one blended flow?
- **Edge behaviour:** calls per request (fan-out multiplier), request weight on the target, read/write share, cache hit ratio on cache Edges, sync vs async (async Edges go through a queue and don't add to the caller's latency), timeout and retry policy, parallel vs sequential with sibling calls.
- **Defaults:** what's inferred without asking. The Brief's `throughput` and `read_write` answers, Notes such as "~10k rps peak", Edge interaction style, and Kind pairs (an API → cache → database chain implies cache-aside).
- **Traffic shapes:** steady, ramp, daily cycle, spike, and a custom curve. Is the ramp-to-failure ("find the breaking point") a shape or a separate action?
- **Storage:** Edge behaviour in the Edge's `customData` (like sizing on Nodes) or only in the Scenario?

## Resolution

Decided 2026-10-15, taking the recommended option throughout.

**Routes.** A **Route** is a named kind of request, such as `GET /product` or `POST /checkout`. It has:
- an entry Node;
- a share of total traffic;
- a latency target (the Brief's `latency` answer by default);
- optional per-Edge overrides of `mult`.

By default it follows every Edge reachable from its entry. The overrides let checkout write where browsing only reads, without drawing two diagrams. Routes belong to the Scenario (ticket 006), not the diagram.

On first open, the runtime creates one Route per entry Node: a Node with no incoming Edge, or one whose callers are `external` Kinds or a Group of kind boundary. The user can split one later. Each Route gets its own line on the dashboard (ticket 007).

**Edge behaviour** is stored on the Production Edge as optional `customData.flow`, since it's part of the design, like Sizing on Nodes:

| Field | Meaning | Default |
|---|---|---|
| `mult` | calls per request served by the caller | 1 |
| `weight` | cost of one call to the target, in typical requests | 1 |
| `op` | `read`, `write` or `mixed` (uses the Brief's read share) | `mixed` |
| `hit` | on an Edge into a cache or CDN: the share answered there; the Edge onward to the origin gets `mult × (1 − hit)` | cache 0.9, CDN 0.7 |
| `async` | from the interaction style: `async` and `stream` don't add to the caller's latency | from style |
| `parallel` | runs alongside the caller's other parallel calls | false |
| `timeout_ms`, `retries`, `backoff` | per call; retries with jittered exponential backoff | 2,000 ms, 1 retry, on |
| `breaker` | circuit breaker on this Edge (ticket 002) | off |

**Defaults are inferred, never asked.** A diagram simulates with zero setup:
- interaction style decides `async`;
- Kind pairs decide `hit` and cache-aside (into a cache, then cache → store gets the miss share);
- the Brief's `read_write` bias sets the read share of `mixed` Edges into data stores (read-heavy 0.9, balanced 0.5, write-heavy 0.2).

**Base traffic** comes from:
1. a Note matching a rate ("~10k rps peak", "500 req/s"), parsed by a small pattern and taken as the peak; otherwise
2. the Brief's `throughput` answer: lt-100 → 50, 100-1k → 500, 1k-10k → 5,000, 10k-plus → 20,000 rps; otherwise
3. 100 rps, with a label saying it's an assumption.

**Traffic shapes:**
- `steady`;
- `daily`: a 24-hour cycle peaking at the base rate, with the trough at 30%;
- `spike`: × N for M minutes at time T;
- `ramp`: from A to B over a duration;
- `custom`: up to 48 points.

**Find breaking point is an action, not a shape.**
- It ramps traffic by 5% every 10 simulated seconds until the first goal breaks: a Route's p99 over its target, more than 1% errors, or a queue's backlog older than 60 s.
- It then pauses and reports the order in which Nodes saturated and where it broke.
- It runs at the current speed and restores the previous traffic shape when dismissed.
