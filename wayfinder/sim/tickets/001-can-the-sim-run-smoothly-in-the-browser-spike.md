---
id: "001"
title: "Can the sim run smoothly in the browser? (spike)"
labels: [wayfinder:task]
status: closed
assignee: claude (spike)
blocked_by: []
parent: map
---

# Can the sim run smoothly in the browser? (spike)

## Question

Before specifying the details, measure the architecture chosen when charting. Build a throwaway page with the pinned Excalidraw (`0.18.0-143b5b6`) and a generated Production Diagram of 50 / 200 / 500 Nodes, and record:

- **Engine tick in a Web Worker:** a dummy fluid model (rates, queues, utilisation per Node and Edge) at 20 ticks per simulated second. Time per tick at each size, and the most simulated time per real second it reaches (target: 100× at 200 Nodes).
- **Probe sampler:** 1k / 5k / 20k probe requests per simulated second traced through the graph with fan-out and retries, recorded into log-bucketed histograms. Time per tick.
- **Snapshot transfer:** worker → page at 10 Hz as transferable typed arrays. Main-thread cost.
- **Overlay:** a Canvas 2D layer over Excalidraw that follows its scroll and zoom (`onScrollChange` / app state), drawing Node heat and up to 5,000 moving dots on Edges at 60 fps. Check it stays aligned while panning, zooming and resizing, and on HiDPI. If Canvas 2D can't hold 60 fps, try the same in WebGL (raw or PixiJS) and compare bundle size.
- **Charts:** 30 live uPlot charts updating at 4 Hz while the overlay animates. Frame rate, and uPlot's lazy-loaded bundle size.
- **Excalidraw while running:** does selecting a Node (for the inspector) and updating a Node's label or colour via `updateScene` cause hitches? Ticket 015 found per-element GPU bitmaps; the overlay must not make that worse.

Output: the numbers, a go/no-go on Canvas 2D vs WebGL and on uPlot, and anything the engine or Sim Mode tickets must respect.

## Resolution

Closed 2026-10-15. **Go**, on the stack chosen when charting: a TypeScript engine in a Web Worker, a **Canvas 2D** overlay (WebGL not needed) and **uPlot** (22.8 KB gzipped, lazy-loaded). Code and raw results: `spikes/sim-001/` ([RESULTS.md](../../../spikes/sim-001/RESULTS.md)). These were not committed, at the user's request.

At 500 Nodes with 30 live charts, the whole diagram on screen (3,195 moving dots) and 100× speed, the page holds 59–60 fps with no long tasks. The overlay stays aligned to 0 px through 40 wheel pans and zooms at DPR 1 and 2. 50 and 200 Nodes hold 60 fps in every case.

Rules the build must follow (the spike found each one):

1. **Sim state never rebuilds or re-renders the canvas.** The first 500-Node run dropped to 45 fps because `useRef(expr)` rebuilt the scene on every 10 Hz render. Keep sim state outside the canvas component, and give `<Excalidraw>` stable, memoised props.
2. **Budget probes per real second.** The engine spends about 200 ns per probe visit. At 100× with fixed probes, the run fell behind (57–86×). The worker scales probes to `budget / speed`, with a floor of 1,000 per simulated second, and reaches 100×.
3. **Charts show windowed percentiles** (a histogram with a 10 s half-life), not raw per-second values. Per-second p99s rest on too few samples.
4. **The overlay follows `snapshotPan`'s in-flight view.** `web/src/snapshotPan.ts` gives Excalidraw the new view only when a gesture ends, so it needs a small subscribe hook for the view during the gesture.
5. **Shed time follows caller timeouts.** The engine reproduces congestion collapse: a backlog wait longer than the caller's timeout turns every call into a timeout plus a retry. That's realistic. By default, a Node sheds requests that have waited longer than its callers' longest timeout (ticket 002).
6. **Sim Mode visuals** (ticket 008): an outline plus a utilisation bar instead of a fill (a fill blurs the label); below 30% zoom, Edge thickness instead of dots; hysteresis on the SLO state.
