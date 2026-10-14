// Runs the Engine off the main thread. Simulated time follows real time × speed;
// each slice runs ticks for at most SLICE_MS, then yields so messages get through.
// Probes adapt to speed so the worker's CPU use stays flat (PROBE_VISITS_PER_REAL_SEC).
import { Engine, FN, NM, RM, SERIES_LEN, TICK_S, type Patch } from "./engine.ts";
import type { System } from "./system.ts";

const SLICE_MS = 12;
const FRAME_HZ = 10;
const SERIES_HZ = 4;

let eng: Engine | null = null;
let playing = true;
let speed = 1;
let probeTarget = 5000; // per simulated second, at 1×
let probeBudget = 0; // probes per real second across all speeds; 0 = fixed probes (bench)
let last = performance.now();
let debt = 0;
let lastFrame = 0;
let lastSeries = 0;
let subNodes: number[] = [];
let subRoutes: number[] = [];
const tickMs: number[] = [];
let simAt = 0;
let realAt = performance.now();
let lagging = false;

function applyProbes() {
  if (!eng) return;
  eng.probesPerSec = probeBudget > 0 ? Math.max(200, Math.min(probeTarget, probeBudget / speed)) : probeTarget;
}

function pump() {
  const now = performance.now();
  if (eng && playing) {
    debt += ((now - last) / 1000) * speed;
    // Never try to catch up more than half a second of real time: fall behind instead.
    if (debt > 0.5 * speed) {
      debt = 0.5 * speed;
      lagging = true;
    }
    const t0 = performance.now();
    while (debt >= TICK_S && performance.now() - t0 < SLICE_MS) {
      const a = performance.now();
      eng.step();
      tickMs.push(performance.now() - a);
      if (tickMs.length > 4000) tickMs.splice(0, 2000);
      debt -= TICK_S;
    }
  }
  last = now;
  if (eng && now - lastFrame >= 1000 / FRAME_HZ) {
    lastFrame = now;
    postFrame();
  }
  if (eng && now - lastSeries >= 1000 / SERIES_HZ) {
    lastSeries = now;
    postSeries();
  }
  setTimeout(pump, 2);
}

function postFrame() {
  const e = eng!;
  const nodes = new Float32Array(e.N * FN);
  const edges = new Float32Array(e.E * 2);
  e.frame(nodes, edges);
  const s = (e.seconds - 1 + SERIES_LEN) % SERIES_LEN;
  const tot = (m: number) => e.routeSeries[(e.R * RM + m) * SERIES_LEN + s];
  const warming = e.warming.length;
  (self as any).postMessage(
    { type: "frame", t: e.t, traffic: e.traffic, cost: e.costPerHour, rate: tot(0), err: tot(1), p50: tot(2), p95: tot(3), p99: tot(4), warming, nodes, edges },
    [nodes.buffer, edges.buffer],
  );
}

/** Copies a ring-buffer series into time order. */
function unroll(src: Float32Array, base: number, out: Float32Array, outOff: number) {
  const e = eng!;
  const start = e.seconds % SERIES_LEN; // oldest
  for (let i = 0; i < SERIES_LEN; i++) out[outOff + i] = src[base + ((start + i) % SERIES_LEN)];
}

function postSeries() {
  const e = eng!;
  const nodes = new Float32Array(subNodes.length * NM * SERIES_LEN);
  subNodes.forEach((n, k) => {
    for (let m = 0; m < NM; m++) unroll(e.nodeSeries, (n * NM + m) * SERIES_LEN, nodes, (k * NM + m) * SERIES_LEN);
  });
  const routes = new Float32Array(subRoutes.length * RM * SERIES_LEN);
  subRoutes.forEach((r, k) => {
    for (let m = 0; m < RM; m++) unroll(e.routeSeries, (r * RM + m) * SERIES_LEN, routes, (k * RM + m) * SERIES_LEN);
  });
  (self as any).postMessage({ type: "series", seconds: e.seconds, subNodes, subRoutes, nodes, routes }, [nodes.buffer, routes.buffer]);
}

self.onmessage = (ev: MessageEvent) => {
  const m = ev.data;
  switch (m.type) {
    case "init":
      eng = new Engine(m.system as System, m.seed ?? 1);
      eng.traffic = m.traffic ?? 1000;
      probeTarget = m.probes ?? 5000;
      probeBudget = m.probeBudget ?? 0;
      applyProbes();
      break;
    case "play":
      playing = m.playing;
      break;
    case "speed":
      speed = m.speed;
      applyProbes();
      break;
    case "traffic":
      eng!.traffic = m.traffic;
      break;
    case "probes":
      probeTarget = m.probes;
      probeBudget = m.probeBudget ?? probeBudget;
      applyProbes();
      break;
    case "patch":
      eng!.patch(m.node, m.patch as Patch);
      break;
    case "subscribe":
      subNodes = m.nodes;
      subRoutes = m.routes;
      break;
    case "stats": {
      // Achieved speed since the last stats call, and tick cost.
      const now = performance.now();
      const achieved = (eng!.t - simAt) / ((now - realAt) / 1000);
      simAt = eng!.t;
      realAt = now;
      const s = [...tickMs].sort((a, b) => a - b);
      const q = (p: number) => s[Math.min(s.length - 1, Math.floor(p * s.length))] ?? 0;
      (self as any).postMessage({ type: "stats", id: m.id, achieved, lagging, tickMean: s.reduce((a, b) => a + b, 0) / (s.length || 1), tickP99: q(0.99), probesPerSec: eng!.probesPerSec });
      tickMs.length = 0;
      lagging = false;
      break;
    }
  }
};

pump();
