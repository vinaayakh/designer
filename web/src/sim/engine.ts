// Fixed-tick fluid simulation with probe requests for tail latency.
//
// Each tick: traffic enters at route entries and flows through the graph in
// topological order. Every Node serves up to its capacity (instances ×
// per-instance rate); the rest waits in a backlog, and non-buffered Nodes shed
// what has waited longer than maxWaitS. Steady-state queueing delay below
// saturation comes from Erlang C (M/M/c). Retries multiply Edge flow by the
// timeout share the probes measured. Autoscaling reacts to smoothed
// utilisation, with a warm-up delay.
//
// Probes are sampled requests walked through the graph, drawing each Node's
// wait and service time from its current state, so p95/p99 reflect fan-out,
// timeouts and retries. They're stratified per route and weighted back to
// traffic share.

import { BUCKETS, bucketOf, quantiles } from "./hist.ts";
import type { System } from "./system.ts";

export const TICK_S = 0.05;
export const SERIES_LEN = 300; // 5 simulated minutes at 1 s
// Per-Node series, in this order.
export const NODE_METRICS = ["rate", "err", "util", "queueS", "inst", "p50", "p95", "p99"] as const;
export const NM = NODE_METRICS.length;
// Per-route and total series.
export const ROUTE_METRICS = ["rate", "err", "p50", "p95", "p99"] as const;
export const RM = ROUTE_METRICS.length;
// Per-Node live state sent for the overlay, in this order.
export const FRAME_NODE = ["util", "rate", "err", "inst", "warming", "down", "queueS"] as const;
export const FN = FRAME_NODE.length;

const QS = [0.5, 0.95, 0.99];
const HALF_LIFE_S = 10; // percentiles shown for "now" cover about the last 10 s
const DECAY = Math.pow(0.5, 1 / HALF_LIFE_S);
const MAX_DEPTH = 16;
const MAX_VISITS = 400;

function mulberry32(seed: number) {
  return () => {
    seed |= 0;
    seed = (seed + 0x6d2b79f5) | 0;
    let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

/** Erlang C: probability an arrival waits, for c servers at offered load a (in servers). */
function erlangC(c: number, a: number): number {
  if (c <= 0) return 1;
  if (a >= c) return 1;
  let b = 1;
  for (let k = 1; k <= c; k++) b = (a * b) / (k + a * b);
  return b / (1 - (a / c) * (1 - b));
}

export type Patch = { count?: number; capPerInstance?: number; medianMs?: number; down?: boolean; scale?: System["nodes"][number]["scale"] | null };

export class Engine {
  sys: System;
  N: number;
  E: number;
  R: number;
  t = 0; // simulated seconds
  traffic = 1000; // total requests/s
  probesPerSec = 5000;
  rand: () => number;
  private spare = NaN;

  // Node state
  count: Float64Array;
  cap: Float64Array;
  med: Float64Array;
  sigma: Float64Array;
  buffered: Uint8Array;
  maxWait: Float64Array;
  down: Uint8Array;
  lam: Float64Array;
  served: Float64Array;
  backlog: Float64Array;
  util: Float64Array;
  dropFrac: Float64Array;
  pwait: Float64Array;
  wqMs: Float64Array; // mean steady-state wait of those who wait
  backlogMs: Float64Array;
  utilEwma: Float64Array;
  errFrac: Float64Array; // measured by probes
  warming: { node: number; readyAt: number }[] = [];
  lastScaleIn: Float64Array;
  topo: Int32Array;
  outStart: Int32Array;
  outList: Int32Array;

  // Edge state
  flow: Float64Array;
  timeoutFrac: Float64Array;
  private eCalls: Float64Array;
  private eTimeouts: Float64Array;

  // Per-second accumulators
  private secTicks = 0;
  private accRate: Float64Array;
  private accUtil: Float64Array;
  private accQueue: Float64Array;
  private nVisits: Float64Array;
  private nErrs: Float64Array;
  private rProbes: Float64Array;
  private rErrs: Float64Array;
  private secHist: Float32Array; // N × BUCKETS, this second's node latencies
  decayHist: Float32Array; // N × BUCKETS, about the last 10 s
  private routeSec: Float32Array; // (R + 1) × BUCKETS; last row is the total
  routeDecay: Float32Array;
  private probeCarry = 0;
  private routeCum: Float64Array;

  // Series (ring buffers, index = second % SERIES_LEN)
  nodeSeries: Float32Array; // N × NM × SERIES_LEN
  routeSeries: Float32Array; // (R + 1) × RM × SERIES_LEN
  seconds = 0;
  costPerHour = 0;

  // Work counters, for benchmarks
  visits = 0;
  probes = 0;

  constructor(sys: System, seed = 1) {
    this.sys = sys;
    const N = (this.N = sys.nodes.length);
    const E = (this.E = sys.edges.length);
    const R = (this.R = sys.routes.length);
    this.rand = mulberry32(seed);
    const f = (n: number) => new Float64Array(n);
    this.count = Float64Array.from(sys.nodes, (n) => n.count);
    this.cap = Float64Array.from(sys.nodes, (n) => n.capPerInstance);
    this.med = Float64Array.from(sys.nodes, (n) => n.medianMs);
    this.sigma = Float64Array.from(sys.nodes, (n) => n.sigma);
    this.buffered = Uint8Array.from(sys.nodes, (n) => (n.buffered ? 1 : 0));
    this.maxWait = Float64Array.from(sys.nodes, (n) => n.maxWaitS ?? 30);
    this.down = new Uint8Array(N);
    this.lam = f(N);
    this.served = f(N);
    this.backlog = f(N);
    this.util = f(N);
    this.dropFrac = f(N);
    this.pwait = f(N);
    this.wqMs = f(N);
    this.backlogMs = f(N);
    this.utilEwma = f(N);
    this.errFrac = f(N);
    this.lastScaleIn = f(N);
    this.flow = f(E);
    this.timeoutFrac = f(E);
    this.eCalls = f(E);
    this.eTimeouts = f(E);
    this.accRate = f(N);
    this.accUtil = f(N);
    this.accQueue = f(N);
    this.nVisits = f(N);
    this.nErrs = f(N);
    this.rProbes = f(R + 1);
    this.rErrs = f(R + 1);
    this.secHist = new Float32Array(N * BUCKETS);
    this.decayHist = new Float32Array(N * BUCKETS);
    this.routeSec = new Float32Array((R + 1) * BUCKETS);
    this.routeDecay = new Float32Array((R + 1) * BUCKETS);
    this.nodeSeries = new Float32Array(N * NM * SERIES_LEN).fill(NaN);
    this.routeSeries = new Float32Array((R + 1) * RM * SERIES_LEN).fill(NaN);
    this.routeCum = f(R);
    let acc = 0;
    sys.routes.forEach((r, i) => (this.routeCum[i] = acc += r.share));

    // Adjacency (CSR) and topological order.
    const deg = new Int32Array(N + 1);
    for (const e of sys.edges) deg[e.from + 1]++;
    for (let i = 0; i < N; i++) deg[i + 1] += deg[i];
    this.outStart = deg;
    this.outList = new Int32Array(E);
    const fill = deg.slice(0, N);
    sys.edges.forEach((e, i) => (this.outList[fill[e.from]++] = i));
    const indeg = new Int32Array(N);
    for (const e of sys.edges) indeg[e.to]++;
    const order: number[] = [];
    for (let i = 0; i < N; i++) if (!indeg[i]) order.push(i);
    for (let k = 0; k < order.length; k++) {
      const n = order[k];
      for (let j = deg[n]; j < deg[n + 1]; j++) {
        const to = sys.edges[this.outList[j]].to;
        if (--indeg[to] === 0) order.push(to);
      }
    }
    if (order.length !== N) throw new Error("call graph has a cycle"); // cycles: engine ticket
    this.topo = Int32Array.from(order);
  }

  patch(n: number, p: Patch) {
    if (p.count != null) this.count[n] = Math.max(0, p.count);
    if (p.capPerInstance != null) this.cap[n] = p.capPerInstance;
    if (p.medianMs != null) this.med[n] = p.medianMs;
    if (p.down != null) this.down[n] = p.down ? 1 : 0;
    if (p.scale !== undefined) this.sys.nodes[n].scale = p.scale ?? undefined;
  }

  step() {
    const dt = TICK_S;
    const { N, sys } = this;
    this.lam.fill(0);
    for (const r of sys.routes) this.lam[r.entry] += this.traffic * r.share;

    let cost = 0;
    for (let k = 0; k < N; k++) {
      const n = this.topo[k];
      const lam = this.lam[n];
      const mu = this.down[n] ? 0 : this.count[n] * this.cap[n];
      cost += this.count[n] * sys.nodes[n].costPerHour;
      const avail = this.backlog[n] + lam * dt;
      const served = Math.min(avail, mu * dt);
      let q = avail - served;
      let dropped = 0;
      if (!this.buffered[n]) {
        const maxQ = mu * this.maxWait[n];
        if (q > maxQ) {
          dropped = q - maxQ;
          q = maxQ;
        }
      }
      this.backlog[n] = q;
      this.served[n] = served / dt;
      this.dropFrac[n] = lam > 0 ? Math.min(1, dropped / (lam * dt)) : 0;
      this.util[n] = mu > 0 ? served / (mu * dt) : lam > 0 ? 1 : 0;
      this.backlogMs[n] = mu > 0 ? (q / mu) * 1000 : 0;
      if (mu > 0 && lam < mu) {
        const c = Math.max(1, Math.round(this.count[n]));
        this.pwait[n] = erlangC(c, lam / this.cap[n]);
        this.wqMs[n] = (1 / (mu - lam)) * 1000;
      } else {
        this.pwait[n] = 1;
        this.wqMs[n] = 0;
      }
      this.utilEwma[n] += (this.util[n] - this.utilEwma[n]) * (dt / 30);
      this.accRate[n] += this.served[n];
      this.accUtil[n] += this.util[n];
      this.accQueue[n] += this.backlogMs[n] / 1000;

      for (let j = this.outStart[n]; j < this.outStart[n + 1]; j++) {
        const ei = this.outList[j];
        const e = sys.edges[ei];
        const tf = this.timeoutFrac[ei];
        let factor = 1;
        let p = 1;
        for (let r = 0; r < e.retries; r++) factor += p *= tf;
        const fl = this.served[n] * e.mult * factor;
        this.flow[ei] = fl;
        this.lam[e.to] += fl;
      }
    }
    (sys.pools ?? []).forEach((p, i) => (cost += (this.poolHosts[i] ?? p.hosts) * p.costPerHostHour));
    this.costPerHour = cost;

    this.runProbes(dt);
    this.t += dt;
    if (++this.secTicks * dt >= 1 - 1e-9) this.rollSecond();
  }

  private gauss(): number {
    if (!Number.isNaN(this.spare)) {
      const s = this.spare;
      this.spare = NaN;
      return s;
    }
    let u = 0;
    while (u === 0) u = this.rand();
    const v = this.rand();
    const m = Math.sqrt(-2 * Math.log(u));
    this.spare = m * Math.sin(2 * Math.PI * v);
    return m * Math.cos(2 * Math.PI * v);
  }

  private runProbes(dt: number) {
    const R = this.R;
    this.probeCarry += this.probesPerSec * dt;
    const n = Math.floor(this.probeCarry);
    this.probeCarry -= n;
    if (!n) return;
    for (let i = 0; i < n; i++) {
      // Stratified: routes take turns, so small routes still get samples.
      const ri = (this.probes + i) % R;
      const route = this.sys.routes[ri];
      // Weight = requests this probe stands for.
      const w = (this.traffic * route.share * dt * R) / n;
      this.visitBudget = MAX_VISITS;
      const lat = this.walk(route.entry, 0, w);
      const b = bucketOf(lat);
      this.routeSec[ri * BUCKETS + b] += w;
      this.routeSec[R * BUCKETS + b] += w;
      this.rProbes[ri] += w;
      this.rProbes[R] += w;
      if (this.lastErr) {
        this.rErrs[ri] += w;
        this.rErrs[R] += w;
      }
    }
    this.probes += n;
  }

  private lastErr = false;
  private visitBudget = 0;

  /** One request (or one call) into Node n. Returns its latency in ms; sets lastErr. */
  private walk(n: number, depth: number, w: number): number {
    this.visits++;
    this.visitBudget--;
    this.nVisits[n] += w;
    if (this.down[n]) {
      this.nErrs[n] += w;
      this.lastErr = true;
      return 1; // connection refused: fast failure
    }
    if (this.rand() < this.dropFrac[n]) {
      this.nErrs[n] += w;
      this.lastErr = true;
      return this.maxWait[n] * 1000; // waited, then shed
    }
    let lat = this.backlogMs[n];
    if (this.rand() < this.pwait[n]) lat += -Math.log(1 - this.rand()) * this.wqMs[n];
    lat += this.med[n] * Math.exp(this.sigma[n] * this.gauss());
    let failed = false;
    let seq = 0;
    let par = 0;
    if (depth < MAX_DEPTH) {
      const edges = this.sys.edges;
      for (let j = this.outStart[n]; j < this.outStart[n + 1]; j++) {
        const ei = this.outList[j];
        const e = edges[ei];
        if (e.async) continue; // enqueued: no wait for the caller
        let calls = Math.floor(e.mult);
        if (this.rand() < e.mult - calls) calls++;
        for (let c = 0; c < calls; c++) {
          let spent = 0;
          let ok = false;
          for (let a = 0; a <= e.retries; a++) {
            this.eCalls[ei] += 1;
            if (this.visitBudget <= 0) {
              ok = true; // budget gone: stop expanding, count as served
              break;
            }
            const sub = this.walk(e.to, depth + 1, w);
            if (sub > e.timeoutMs) {
              this.eTimeouts[ei] += 1;
              spent += e.timeoutMs;
              continue;
            }
            spent += sub;
            if (!this.lastErr) {
              ok = true;
              break;
            }
          }
          if (!ok) failed = true;
          if (e.par) par = Math.max(par, spent);
          else seq += spent;
        }
      }
    }
    lat += seq + par;
    this.secHist[n * BUCKETS + bucketOf(lat)] += w;
    if (failed) this.nErrs[n] += w;
    this.lastErr = failed;
    return lat;
  }

  private q3 = new Float32Array(3);

  private rollSecond() {
    const { N, R, E, sys } = this;
    const ticks = this.secTicks;
    this.secTicks = 0;
    const s = this.seconds++ % SERIES_LEN;
    const q3 = this.q3;

    for (let n = 0; n < N; n++) {
      const off = n * BUCKETS;
      // Decay the window, then add this second.
      for (let i = 0; i < BUCKETS; i++) {
        this.decayHist[off + i] = this.decayHist[off + i] * DECAY + this.secHist[off + i];
        this.secHist[off + i] = 0;
      }
      quantiles(this.decayHist, off, QS, q3, 0);
      const base = n * NM * SERIES_LEN + s;
      const v = this.nVisits[n];
      this.errFrac[n] = v > 0 ? this.nErrs[n] / v : 0;
      this.nodeSeries[base + 0 * SERIES_LEN] = this.accRate[n] / ticks;
      this.nodeSeries[base + 1 * SERIES_LEN] = this.errFrac[n];
      this.nodeSeries[base + 2 * SERIES_LEN] = this.accUtil[n] / ticks;
      this.nodeSeries[base + 3 * SERIES_LEN] = this.accQueue[n] / ticks;
      this.nodeSeries[base + 4 * SERIES_LEN] = this.count[n];
      this.nodeSeries[base + 5 * SERIES_LEN] = q3[0];
      this.nodeSeries[base + 6 * SERIES_LEN] = q3[1];
      this.nodeSeries[base + 7 * SERIES_LEN] = q3[2];
      this.accRate[n] = this.accUtil[n] = this.accQueue[n] = this.nVisits[n] = this.nErrs[n] = 0;
    }
    for (let r = 0; r <= R; r++) {
      const off = r * BUCKETS;
      for (let i = 0; i < BUCKETS; i++) {
        this.routeDecay[off + i] = this.routeDecay[off + i] * DECAY + this.routeSec[off + i];
        this.routeSec[off + i] = 0;
      }
      quantiles(this.routeDecay, off, QS, q3, 0);
      const base = r * RM * SERIES_LEN + s;
      const rate = r < R ? this.traffic * sys.routes[r].share : this.traffic;
      this.routeSeries[base + 0 * SERIES_LEN] = rate;
      this.routeSeries[base + 1 * SERIES_LEN] = this.rProbes[r] > 0 ? this.rErrs[r] / this.rProbes[r] : 0;
      this.routeSeries[base + 2 * SERIES_LEN] = q3[0];
      this.routeSeries[base + 3 * SERIES_LEN] = q3[1];
      this.routeSeries[base + 4 * SERIES_LEN] = q3[2];
      this.rProbes[r] = this.rErrs[r] = 0;
    }
    for (let e = 0; e < E; e++) {
      const c = this.eCalls[e];
      const tf = c > 0 ? this.eTimeouts[e] / c : 0;
      this.timeoutFrac[e] += (tf - this.timeoutFrac[e]) * 0.5;
      this.eCalls[e] = this.eTimeouts[e] = 0;
    }
    this.autoscale();
  }

  // Capacity pools (ECS on EC2, EKS node groups): two-level scaling.
  poolHosts: number[] = [];
  poolWarming: { pool: number; readyAt: number }[] = [];
  unscheduled: Float64Array = new Float64Array(0); // tasks/pods wanted but not placed: no room on the pool
  private poolsReady = false;

  private initPools() {
    this.poolHosts = (this.sys.pools ?? []).map((p) => p.hosts);
    this.unscheduled = new Float64Array(this.N);
    this.poolsReady = true;
  }

  /** vCPU free on a pool for new tasks (running + warming tasks hold their vCPU). */
  private poolFree(p: number) {
    const pool = this.sys.pools![p];
    let used = 0;
    for (let n = 0; n < this.N; n++) if (this.sys.nodes[n].pool === p) used += this.count[n] * (this.sys.nodes[n].vcpu ?? 1);
    for (const w of this.warming) if (this.sys.nodes[w.node].pool === p) used += this.sys.nodes[w.node].vcpu ?? 1;
    return this.poolHosts[p] * pool.vcpuPerHost - used;
  }

  /** Starts up to `want` new instances of Node n; on a pool, only as many as fit. Returns how many started. */
  private start(n: number, want: number) {
    const nd = this.sys.nodes[n];
    let k = want;
    if (nd.pool != null) k = Math.max(0, Math.min(want, Math.floor(this.poolFree(nd.pool) / (nd.vcpu ?? 1))));
    for (let i = 0; i < k; i++) this.warming.push({ node: n, readyAt: this.t + (nd.scale?.warmupS ?? 30) });
    return k;
  }

  private autoscale() {
    const { t, sys } = this;
    if (!this.poolsReady) this.initPools();
    // Instances and hosts that finished warming up join.
    this.warming = this.warming.filter((w) => {
      if (w.readyAt > t) return true;
      this.count[w.node]++;
      return false;
    });
    this.poolWarming = this.poolWarming.filter((w) => {
      if (w.readyAt > t) return true;
      this.poolHosts[w.pool]++;
      return false;
    });
    // Unplaced tasks/pods get placed as soon as room appears (every second, like a scheduler).
    for (let n = 0; n < this.N; n++) if (this.unscheduled[n] > 0) this.unscheduled[n] -= this.start(n, this.unscheduled[n]);
    if (this.seconds % 15) return; // evaluate every 15 s, like a metrics-based autoscaler
    for (let n = 0; n < this.N; n++) {
      const sc = sys.nodes[n].scale;
      if (!sc || this.down[n]) continue;
      const pending = this.warming.reduce((a, w) => a + (w.node === n ? 1 : 0), 0) + this.unscheduled[n];
      // cpu: target tracking on utilisation, which caps at 100% (under-reads demand when saturated).
      // requests: target tracking on requests per instance (e.g. ALBRequestCountPerTarget) sees real demand.
      const desired = Math.min(sc.max, Math.max(sc.min, Math.ceil(sc.metric === "requests" ? this.lam[n] / (this.cap[n] * sc.target) : (this.count[n] * this.utilEwma[n]) / sc.target)));
      if (desired > this.count[n] + pending) {
        const want = desired - this.count[n] - pending;
        this.unscheduled[n] += want - this.start(n, want);
      } else if (desired < this.count[n] && !pending && t - this.lastScaleIn[n] >= sc.cooldownS) {
        this.count[n] = desired;
        this.lastScaleIn[n] = t;
      }
    }
    // Cluster autoscaler: add hosts for unplaced vCPU (EC2 boot + join takes warmupS).
    (sys.pools ?? []).forEach((pool, p) => {
      if (!pool.scale) return;
      let need = 0;
      for (let n = 0; n < this.N; n++) if (sys.nodes[n].pool === p) need += this.unscheduled[n] * (sys.nodes[n].vcpu ?? 1);
      const coming = this.poolWarming.filter((w) => w.pool === p).length;
      const add = Math.min(pool.scale.max - this.poolHosts[p] - coming, Math.ceil(need / pool.vcpuPerHost) - coming);
      for (let i = 0; i < add; i++) this.poolWarming.push({ pool: p, readyAt: t + pool.scale.warmupS });
    });
  }

  /** Live per-Node and per-Edge state for the overlay. */
  frame(nodes: Float32Array, edges: Float32Array) {
    const pend = new Float64Array(this.N);
    for (const w of this.warming) pend[w.node]++;
    for (let n = 0; n < this.N; n++) {
      const o = n * FN;
      nodes[o] = this.util[n];
      nodes[o + 1] = this.served[n];
      nodes[o + 2] = Math.max(this.errFrac[n], this.dropFrac[n]);
      nodes[o + 3] = this.count[n];
      nodes[o + 4] = pend[n];
      nodes[o + 5] = this.down[n];
      nodes[o + 6] = this.backlogMs[n] / 1000;
    }
    for (let e = 0; e < this.E; e++) {
      edges[e * 2] = this.flow[e];
      edges[e * 2 + 1] = this.timeoutFrac[e];
    }
  }
}
