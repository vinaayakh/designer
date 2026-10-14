// Runs the live simulation off the main thread (sim ticket 001): simulated time
// follows real time × speed in short slices, probes are budgeted per real second,
// and the overlay state is posted at 10 Hz. Headless runs (baseline, test) use
// their own engines and don't disturb the live one.
import { Engine, FN, RM, SERIES_LEN, TICK_S, type Patch } from "./engine.ts";
import { runSummary } from "./run.ts";
import type { System } from "./system.ts";

const SLICE_MS = 12;
const PROBE_BUDGET = 100_000; // probes per real second, across all speeds

let eng: Engine | null = null;
let playing = true;
let speed = 1;
let last = performance.now();
let debt = 0;
let lastFrame = 0;

const post = (m: unknown, t?: Transferable[]) => (self as unknown as Worker).postMessage(m, t ?? []);

function pump() {
  const now = performance.now();
  if (eng && playing) {
    debt = Math.min(debt + ((now - last) / 1000) * speed, 0.5 * speed);
    const t0 = performance.now();
    while (debt >= TICK_S && performance.now() - t0 < SLICE_MS) {
      eng.step();
      debt -= TICK_S;
    }
  }
  last = now;
  if (eng && now - lastFrame >= 100) {
    lastFrame = now;
    const nodes = new Float32Array(eng.N * FN);
    const edges = new Float32Array(eng.E * 2);
    eng.frame(nodes, edges);
    const s = (eng.seconds - 1 + SERIES_LEN) % SERIES_LEN;
    const tot = (m: number) => eng!.routeSeries[(eng!.R * RM + m) * SERIES_LEN + s];
    post({ type: "frame", t: eng.t, traffic: eng.traffic, cost: eng.costPerHour, rate: tot(0), err: tot(1), p50: tot(2), p95: tot(3), p99: tot(4), nodes, edges }, [nodes.buffer, edges.buffer]);
  }
  setTimeout(pump, 2);
}

self.onmessage = (ev: MessageEvent) => {
  const m = ev.data;
  switch (m.type) {
    case "init":
      eng = new Engine(m.system as System, m.seed ?? 1);
      eng.traffic = m.traffic;
      eng.probesPerSec = Math.max(1000, Math.min(5000, PROBE_BUDGET / speed));
      debt = 0;
      break;
    case "play":
      playing = m.playing;
      break;
    case "speed":
      speed = m.speed;
      if (eng) eng.probesPerSec = Math.max(1000, Math.min(5000, PROBE_BUDGET / speed));
      break;
    case "traffic":
      if (eng) eng.traffic = m.traffic;
      break;
    case "patch":
      eng?.patch(m.node, m.patch as Patch);
      break;
    case "run":
      post({ type: "summary", id: m.id, summary: runSummary(m.system as System, m.opts) });
      break;
  }
};

pump();
