# Spike sim-001: can the sim run smoothly in the browser?

Ticket: [wayfinder/sim/tickets/001](../../wayfinder/sim/tickets/001-can-the-sim-run-smoothly-in-the-browser-spike.md).
Machine: Ryzen 5 2600, 16 GB, RX 6700, Edge 155 (headed, real GPU), 1600×900; the same setup as ticket 015.
Pinned: `@excalidraw/excalidraw@0.18.0-143b5b6`, `uplot@1.6.32`, React 19.3.0, Vite 8.3.4.

**Verdict: go.** TypeScript in a Web Worker, a Canvas 2D overlay and uPlot hold 60 fps at 500 Nodes with 30 live
charts at 100× speed. WebGL isn't needed.

## How to run

```
npm install
npx vite build
node bench/engine.ts           # engine cost in Node (results/engine.json)
node bench/run.mjs             # browser matrix (results/browser.json); opens Edge windows
node bench/memo.mjs            # 500 Nodes after the render fix (results/memo.json)
node bench/profiles-check.ts   # ticket 011: starter Profiles on three systems (results/profiles-check.json)
node bench/pools-check.ts      # ticket 014: ECS on Fargate vs EC2 capacity pools, CPU vs request scaling (results/pools-check.json)
```

URL parameters for the page (`dist/index.html` behind `bench/serve.mjs`): `domains` (×10 Nodes), `charts`,
`probes`, `budget`, `traffic`, `dashboard=0`, `memo=0`.

## What was built

- `src/engine.ts`: a fixed 50 ms tick fluid model (capacity, backlog, shedding past `maxWaitS`, Erlang C waiting
  below saturation, retries scaled by measured timeout share, autoscaling with warm-up), plus probe requests
  walked through the graph for p50/p95/p99. Latency goes into log-bucketed histograms (±1%, 743 buckets),
  with per-second rings and a 10 s half-life window.
- `src/sim.worker.ts`: runs the engine at real time × speed in 12 ms slices, posts a frame at 10 Hz and
  subscribed series at 4 Hz as transferable typed arrays, and adapts the probe rate to speed (`budget`).
- `src/overlay.ts`: a Canvas 2D layer above Excalidraw, drawn in scene coordinates with Excalidraw's own
  transform. It shows a heat outline, a utilisation bar and badges per Node, and dots along each arrow with a
  log-scaled count capped at 12 per Edge.
- `src/dashboard.ts`: uPlot charts (latency with an SLO line, traffic and errors, per-Node utilisation and p99),
  lazy-loaded, with event markers drawn by a plugin.
- `src/main.tsx`: a rough Sim Mode layout (control bar, read-only canvas with click hit-testing, inspector,
  docked dashboard) and the `window.__bench` hooks.

## Engine cost (Node, no browser): `results/engine.json`

| Nodes | Probes per simulated s | ms per simulated s | Fastest speed |
|---|---|---|---|
| 50 | 0 / 1k / 5k / 20k | 0.5 / 1.4 / 6.4 / 25 | 2,084× / 701× / 157× / 40× |
| 200 | 0 / 1k / 5k / 20k | 1.1 / 3.3 / 11.4 / 36 | 953× / 303× / 88× / 28× |
| 500 | 0 / 1k / 5k / 20k | 2.9 / 5.7 / 18.6 / 39 | 348× / 176× / 54× / 26× |

- The flow model costs nothing that matters. **Probes are the cost**: about 200 ns per Node visit, about 6 visits
  per probe here.
- **Memory:** 8 MB at 500 Nodes (5 minutes of series at 1 s, plus histograms).

## Browser: `results/browser.json`, `results/memo.json`

After the render fix below (500 Nodes, `memo=1`):

| Case | fps | p99 frame | Long tasks | Dots drawn | Speed reached |
|---|---|---|---|---|---|
| Normal view, no charts | 59.9 | 16.8 ms | 0 | 580 | 1× |
| Whole diagram on screen, no charts | 59.3 | 16.8 ms | 0 | 3,195 | 1× |
| Whole diagram, 100× | 59.9 | 16.9 ms | 0 | 3,195 | 99× |
| Normal view, 30 charts | 59.9 | 16.9 ms | 0 | 348 | 1× |
| Whole diagram, 30 charts | 59.0 | 17.0 ms | 0 | 3,195 | 1× |
| Whole diagram, 30 charts, 100× | 59.1 | 33.3 ms | 0 | 3,195 | 100× |
| Wheel pan and zoom at DPR 2, 30 charts | 59.1 | 33.2 ms | 1 (61 ms) | 820 | 1× |

- **50 and 200 Nodes** held 60 fps in every case of the first run.
- **Alignment:** overlay against `sceneCoordsToViewportCoords` for every Node, after each of 40 wheel pans and zooms:
  **0 px** worst case, at DPR 1 and 2.
- **Speed:** with fixed probes (5k per simulated second), 100× fell behind (86× at 200 Nodes, 57× at 500). With a
  probe budget of 100k per real second, the worker drops probes to 1k per simulated second at 100× and reaches 100×.
- **Bundle:** the worker is 4.0 KB gzipped; the dashboard chunk (uPlot plus charts) is 22.8 KB gzipped plus 0.7 KB of CSS,
  loaded only when Sim Mode opens.

## Findings

1. **Go on Canvas 2D; WebGL isn't needed.** At 3,195 dots the overlay draws in well under a frame.
2. **Live sim state must never rebuild or re-render the scene.** The first 500-Node run showed 45–53 fps with
   60–120 ms long tasks. A CPU profile traced them to `useRef(sceneFor(sys))`: the argument is evaluated on every
   render, so each 10 Hz frame message converted 1,700 elements again. With lazy initialisers it's 60 fps. Memoising
   the `<Excalidraw>` element adds a little headroom (54 → 59 fps in the heaviest case). Rule for the build: sim
   state lives outside the canvas component, and the canvas gets stable props.
3. **Probe budget per real second, not per simulated second.** The engine adapts its probe rate to speed.
4. **p99 needs enough samples.** At 5k probes/s spread over 50 routes, a per-second p99 per route rests on about
   100 samples. Charts therefore show the 10 s window (decayed histogram), not raw per-second values, and per-Node
   p99 is only meaningful for Nodes with steady traffic.
5. **The engine reproduces congestion collapse.** When a Node's backlog wait (5 s) is longer than its caller's
   timeout (2 s), every call times out and retries, and errors jump from 0 to 100% within one ramp step. That's
   realistic, and it's what users should learn to spot. It also means a Node's default shed time should follow its
   callers' timeouts (engine ticket).
6. **The real app's snapshot panning needs a hook.** `web/src/snapshotPan.ts` moves a copied frame with CSS during
   a gesture and only hands Excalidraw the final view at the end, so `onScrollChange` doesn't fire mid-gesture. The
   overlay must follow snapshotPan's in-flight view (export a subscribe hook), or it will lag during pans.
   This spike used plain Excalidraw panning.
7. **Design notes for Sim Mode** (ticket 008):
   - A heat fill over a Node blurs its label, so use an outline plus a utilisation bar.
   - At 20% zoom the dots crowd the Edges; below about 30% zoom, draw flow as Edge thickness instead.
   - The SLO badge flickers around the target; it needs hysteresis.

## Not measured

Laptop GPUs; Firefox and Safari; the real app's snapshot panning; graphs with cycles (the spike's engine rejects
them); memory beyond 5 minutes of series.
