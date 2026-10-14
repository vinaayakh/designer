// Engine cost without a browser: time per simulated second at 50/200/500 Nodes
// and 1k/5k/20k probes per simulated second, plus sanity checks on the numbers.
// Run: node bench/engine.ts
import { Engine, NM, RM, SERIES_LEN, TICK_S } from "../src/engine.ts";
import { buildSystem } from "../src/system.ts";
import { writeFileSync } from "node:fs";

const pct = (xs: number[], p: number) => {
  const s = [...xs].sort((a, b) => a - b);
  return s[Math.min(s.length - 1, Math.floor(p * s.length))];
};
const r2 = (x: number) => Math.round(x * 100) / 100;
const ticksPerSec = Math.round(1 / TICK_S);

const rows: any[] = [];
for (const domains of [5, 20, 50]) {
  for (const probes of [0, 1000, 5000, 20000]) {
    const sys = buildSystem(domains);
    const eng = new Engine(sys, 7);
    eng.traffic = 200 * domains; // ~35% load on the APIs
    eng.probesPerSec = probes;
    for (let i = 0; i < 30 * ticksPerSec; i++) eng.step(); // warm-up (JIT + state)
    const v0 = eng.visits;
    const perSec: number[] = [];
    const tick: number[] = [];
    for (let s = 0; s < 60; s++) {
      const a = performance.now();
      for (let i = 0; i < ticksPerSec; i++) {
        const b = performance.now();
        eng.step();
        tick.push(performance.now() - b);
      }
      perSec.push(performance.now() - a);
    }
    const msPerSimSec = perSec.reduce((a, b) => a + b, 0) / perSec.length;
    rows.push({
      nodes: sys.nodes.length,
      edges: sys.edges.length,
      probesPerSimSec: probes,
      visitsPerProbe: probes ? r2((eng.visits - v0) / (probes * 60)) : 0,
      tickMeanMs: r2(tick.reduce((a, b) => a + b, 0) / tick.length),
      tickP99Ms: r2(pct(tick, 0.99)),
      msPerSimSecond: r2(msPerSimSec),
      maxSpeedX: Math.round(1000 / msPerSimSec),
      engineMB: r2(
        (eng.nodeSeries.byteLength + eng.routeSeries.byteLength + eng.decayHist.byteLength * 2 + eng.routeDecay.byteLength * 2) / 1048576,
      ),
    });
    console.log(JSON.stringify(rows.at(-1)));
  }
}

// Sanity: one domain, ramp traffic until the first SLO breaks; report the saturation order.
function ramp() {
  const sys = buildSystem(1);
  const eng = new Engine(sys, 3);
  eng.probesPerSec = 5000;
  for (const n of sys.nodes) n.scale = undefined; // fixed sizing for a clean ramp
  const order: { node: string; atRps: number }[] = [];
  const seen = new Set<number>();
  let broke: any = null;
  const at = (series: Float32Array, base: number) => series[base + ((eng.seconds - 1) % SERIES_LEN)];
  for (let rps = 100; rps <= 6000 && !broke; rps += 100) {
    eng.traffic = rps;
    for (let i = 0; i < 20 * ticksPerSec; i++) eng.step();
    for (let n = 0; n < eng.N; n++) {
      if (!seen.has(n) && eng.util[n] > 0.95) {
        seen.add(n);
        order.push({ node: sys.nodes[n].id, atRps: rps });
      }
    }
    const R = eng.R; // total row
    const p99 = at(eng.routeSeries, (R * RM + 4) * SERIES_LEN);
    const err = at(eng.routeSeries, (R * RM + 1) * SERIES_LEN);
    const p50 = at(eng.routeSeries, (R * RM + 2) * SERIES_LEN);
    if (rps === 100) console.log(`low load: p50 ${r2(p50)} ms, p99 ${r2(p99)} ms`);
    if (p99 > 300 || err > 0.01) broke = { rps, p50: r2(p50), p99: r2(p99), err: r2(err) };
  }
  return { saturationOrder: order, firstSloBreak: broke };
}
const sanity = ramp();
console.log(JSON.stringify(sanity, null, 1));
writeFileSync(new URL("../results/engine.json", import.meta.url), JSON.stringify({ rows, sanity }, null, 1));
void NM;
