---
id: "007"
title: "How are metrics collected and shown on the dashboard?"
labels: [wayfinder:grilling]
status: closed
assignee: claude (recommended options, for vinu's review)
blocked_by: ["001", "002"]
parent: map
---

# How are metrics collected and shown on the dashboard?

## Question

The monitoring dashboard shows traffic, error rate, response times (p50, p95, p99) and cost while the simulation runs. Settled when charting: probe requests give tail latency, latencies are kept in histograms, and percentiles are read from merged histograms. Decide the rest.

- **Metric set:** per route (rate, errors, p50/p95/p99 against the SLO), per Node in the RED view (rate, errors, duration), per resource in the USE view (utilisation, saturation as queue depth, errors), per Edge (flow, errors, retries), totals (cost per hour, error budget burn).
- **Histogram:** bucket scheme (log-bucketed, HDR- or DDSketch-style), relative error target, memory per series.
- **Retention:** ring buffers in the worker. Resolution tiers (e.g. 1 s for the last 5 simulated minutes, rolled up after that), and the memory cap.
- **Transfer:** only series for charts on screen, at about 4 Hz, as transferable buffers.
- **Layout:** overview tiles (current value, sparkline, red when over target), route charts with the SLO line, a sortable service table, and a detail view for one Node. A docked panel, a separate tab, or both?
- **Event markers:** traffic changes, scripted events, failures and the user's own edits on every chart's time axis.
- **Linking to the canvas:** hovering a chart highlights its Node; clicking a Node filters the dashboard; scrubbing the time axis freezes the canvas overlay at that moment (needs per-tick snapshots kept).
- **Compare runs:** overlay the same Scenario before and after a change, with the same seed.
- **Charting:** uPlot, lazy-loaded with Sim Mode, if spike 001 confirms it. Chart styling follows the `dataviz` skill.

## Resolution

Decided 2026-10-15, taking the recommended option throughout. Spike 001 confirmed uPlot and the histogram design.

**Metrics** (the RED and USE views):

| Scope | Metrics |
|---|---|
| Totals | requests/s, error %, p50/p95/p99, cost per hour, error-budget burn rate against the Brief's availability target |
| Route | requests/s, error %, p50/p95/p99 against its latency target |
| Node, RED | rate, error %, duration p50/p95/p99 (what its callers see, including downstream calls) |
| Node, USE | utilisation, saturation (wait time from backlog), errors, instances (ready and warming) |
| Queue | depth and **backlog age**, a first-class goal: async pipelines break silently otherwise (ticket 011) |
| Edge | flow, timeout and error share, retries per second, breaker state |

**Histograms.**
- Log buckets with γ = 1.02 (±1%) from 0.05 ms to 120 s: 743 buckets, about 3 KB per series. Values are weighted by traffic share (probes are stratified).
- Each series keeps one scratch histogram per retention tier. At the end of each window it's read for p50/p95/p99, then reset, so every stored point is exact for its window.
- A 10 s half-life histogram gives the "now" values on tiles and the overlay.
- Percentiles are never averaged.

**Retention** (simulated time) uses ring buffers in the worker:
- 1 s points for 5 minutes;
- 10 s points for 1 hour;
- 1 min points for 24 hours.

That comes to about 25 MB at 500 Nodes. Older data falls off, and the saved run keeps the 1-minute tier (ticket 006).

**Transfer.**
- Overlay state: 10 Hz.
- Chart series: 4 Hz, and only for charts on screen.
- Both go as transferable typed arrays, as measured in the spike.

**Layout:** a dock under the canvas, collapsible and resizable, so the canvas and charts are seen together. Top to bottom:
1. **Overview tiles:** traffic, errors, p95, p99, cost, and backlog age when queues exist. Each shows the current value, a sparkline and its target.
2. **Route charts:** p50/p95/p99 with the latency target as a dashed line, plus traffic and errors.
3. **Service table:** one row per Node, sortable by utilisation, p99 or errors, worst first by default.
4. **Node detail**, when a Node is selected: its RED and USE charts.

**SLO state has hysteresis.** A goal turns red after 3 consecutive simulated seconds over its target and green after 10 seconds under, so tiles and the pass/fail pill don't flicker around the line (spike 001).

**Event markers:** traffic changes, scripted events, failures and **every user edit**, drawn on every chart's time axis with a label on hover.

**Linking.**
- Hovering a chart or table row highlights its Node on the canvas; clicking a Node shows its detail.
- Time scrubbing comes later. It needs per-second overlay snapshots (about 14 KB each at 500 Nodes, roughly 4 MB per 5 minutes), which is cheap enough, but not needed for v1.

**Compare runs** is in v1, because it is the before-and-after that shows whether a change helped. The user marks a run as baseline. The next run of the same Scenario with the same seed overlays the baseline as faint series on every chart, and the tiles show deltas ("p99 −42%").

**Charts:** uPlot (22.8 KB gzipped, lazy chunk), styled by the `dataviz` skill when built. Axes show simulated time as m:ss from the start of the run.
