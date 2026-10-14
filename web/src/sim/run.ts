// A headless run of a Scenario, as fast as the CPU allows: steady traffic for
// the numbers at the Scenario's rate, then a ramp to the breaking point. Same
// seed, same system → same summary, so runs compare fairly (ticket 002).
import { Engine, RM, SERIES_LEN, TICK_S } from "./engine.ts";
import { BUCKETS, quantiles } from "./hist.ts";
import type { System } from "./system.ts";
import type { RunSummary } from "./resolve.ts";

const TICKS = Math.round(1 / TICK_S);

function seconds(e: Engine, n: number, each?: () => void) {
  for (let s = 0; s < n; s++) {
    for (let i = 0; i < TICKS; i++) {
      e.step();
      each?.();
    }
  }
}

/** The last second's value of a total-route metric (see engine.ROUTE_METRICS). */
const total = (e: Engine, m: number) => e.routeSeries[(e.R * RM + m) * SERIES_LEN + ((e.seconds - 1 + SERIES_LEN) % SERIES_LEN)];

export function runSummary(sys: System, o: { seed: number; trafficRps: number; sloMs: number; costExcludes: number }): RunSummary {
  // 1. Steady: 60 s to settle (autoscaling included), 30 s measured.
  const e = new Engine(sys, o.seed);
  e.probesPerSec = 4000;
  e.traffic = o.trafficRps;
  seconds(e, 60);
  const util = new Float64Array(e.N);
  let errSum = 0;
  for (let s = 0; s < 30; s++) {
    seconds(e, 1, () => {
      for (let n = 0; n < e.N; n++) util[n] += e.util[n];
    });
    errSum += total(e, 1);
  }
  const q = new Float32Array(3);
  quantiles(e.routeDecay, e.R * BUCKETS, [0.5, 0.95, 0.99], q, 0);
  const utilAtTraffic: Record<string, number> = {};
  sys.nodes.forEach((n, i) => (utilAtTraffic[n.id] = util[i] / (30 * TICKS)));

  // 2. Ramp: +10% every 20 simulated seconds until p99 or errors break the targets.
  const r = new Engine(sys, o.seed);
  r.probesPerSec = 3000;
  let rps = Math.max(1, o.trafficRps * 0.25);
  let breakRps: number | null = null;
  let breakReason = "";
  let busiest: RunSummary["busiest"] = [];
  const limit = Math.max(o.trafficRps * 100, 1000);
  while (rps <= limit) {
    r.traffic = rps;
    seconds(r, 20);
    const p99 = total(r, 4), err = total(r, 1);
    if (p99 > o.sloMs || err > 0.01) {
      breakRps = rps;
      breakReason = err > 0.01 ? `${(err * 100).toFixed(1)}% errors` : `p99 ${Math.round(p99)} ms over the ${o.sloMs} ms target`;
      busiest = sys.nodes
        .map((n, i) => ({ id: n.id, label: n.label, util: r.util[i], src: n.kind.startsWith("external.") }))
        .filter((x) => !x.src)
        .sort((a, b) => b.util - a.util)
        .slice(0, 3)
        .map(({ id, label, util }) => ({ id, label, util }));
      break;
    }
    rps *= 1.1;
  }

  return {
    at: new Date().toISOString(), trafficRps: o.trafficRps, sloMs: o.sloMs,
    p50: q[0], p95: q[1], p99: q[2], errPct: (errSum / 30) * 100, costPerHour: e.costPerHour, costExcludes: o.costExcludes,
    breakRps: breakRps == null ? null : Math.round(breakRps), breakReason: breakRps == null ? `held to ${Math.round(limit)} rps` : breakReason,
    busiest, utilAtTraffic,
  };
}
