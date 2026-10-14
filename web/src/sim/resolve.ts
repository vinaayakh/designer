// Production Diagram + Behaviour Profiles + Brief + Scenario → what the engine
// runs. Everything the import has to guess is reported as a note, so the user
// can see and correct it in the inspector (the Scenario keeps the corrections).
import type { Graph, GNode } from "../types.ts";
import type { EdgeSpec, NodeSpec, Route, System } from "./system.ts";

// ---- Scenario: the user's corrections and changes, on top of one Production version.

export type SizingOverride = { count?: number; size?: string; scale?: { min: number; max: number; target: number } | null; simAs?: string };
export type FlowOverride = { mult?: number; timeoutMs?: number; retries?: number };
export type AddedCache = { ref: string; label: string; from: string; to: string; hit: number };
export type RunSummary = {
  at: string; trafficRps: number; sloMs: number;
  p50: number; p95: number; p99: number; errPct: number; costPerHour: number; costExcludes: number;
  breakRps: number | null; breakReason: string; busiest: { id: string; label: string; util: number }[];
  utilAtTraffic: Record<string, number>;
};
export type Scenario = {
  name: string; base: number; seed: number; trafficRps: number;
  sizing: Record<string, SizingOverride>; flow: Record<string, FlowOverride>; added: AddedCache[];
  baseline?: RunSummary; last?: RunSummary;
};

export type Profiles = Record<string, any>;
export type Note = { id?: string; text: string };
export type Resolved = {
  system: System;
  ids: string[]; // engine node index → diagram element id (or +ref)
  edgeIds: string[]; // engine edge index → diagram element id (or +ref:…)
  sizing: Record<string, Sizing>; // what each simulated Node runs as (after overrides)
  excluded: { id: string; label: string; why: string }[];
  notes: Note[];
  costExcludes: number; // simulated Nodes with no hourly price
  sloMs: number;
  trafficRps: number;
  cacheService?: string; // what an added cache is on this cloud
};
export type Sizing = {
  kind: string; simAs?: string; model: string; count: number; size?: string; sizes: SizeOpt[];
  scale?: { min: number; max: number; target: number }; fromDiagram: string[]; capPerInstance: number;
};
export type SizeOpt = { id: string; vcpu?: number; ram_gb?: number; price_hour?: number };

const T = (r: any): number => (r == null ? NaN : typeof r === "number" ? r : r.typical);

/** Brief answers → defaults (ticket 005). */
export function briefDefaults(answers: Record<string, string>) {
  const traffic: Record<string, number> = { "lt-100": 50, "100-1k": 500, "1k-10k": 5000, "10k-plus": 20000 };
  const slo: Record<string, number> = { "lt-50ms": 50, "lt-300ms": 300, seconds: 1000 };
  const read: Record<string, number> = { "read-heavy": 0.9, balanced: 0.5, "write-heavy": 0.2 };
  return {
    trafficRps: traffic[answers.throughput] ?? 100,
    trafficAssumed: !(answers.throughput in traffic),
    sloMs: slo[answers.latency] ?? 300,
    readShare: read[answers.read_write_bias] ?? 0.5,
  };
}

export function newScenario(base: number, answers: Record<string, string>): Scenario {
  return { name: "Baseline", base, seed: 1234, trafficRps: briefDefaults(answers).trafficRps, sizing: {}, flow: {}, added: [] };
}

// ---- Sizing from the diagram's free-text attributes (e.g. tasks: "2-4 across 2 AZs").

const COUNT_KEYS = ["instances", "tasks", "count", "replicas", "nodes", "pods", "brokers", "shards"];
const SIZE_KEYS = ["instance", "instance_type", "instance_class", "size", "node_type"];

export function sizingFromAttrs(attrs: Record<string, string> = {}) {
  const out: { count?: number; scale?: { min: number; max: number; target: number }; size?: string; keys: string[] } = { keys: [] };
  for (const k of COUNT_KEYS) {
    const v = attrs[k];
    if (!v) continue;
    const m = /(\d+)\s*(?:-|–|to)\s*(\d+)/.exec(v);
    if (m) {
      out.count = +m[1];
      out.scale = { min: +m[1], max: +m[2], target: 0.6 };
    } else if (/\d+/.test(v)) out.count = +/\d+/.exec(v)![0];
    out.keys.push(k);
    break;
  }
  if (attrs.autoscaling) {
    const m = /(\d+)\s*(?:-|–|to)\s*(\d+)(?:.*?(\d+)\s*%)?/.exec(attrs.autoscaling);
    if (m) {
      out.scale = { min: +m[1], max: +m[2], target: m[3] ? +m[3] / 100 : 0.6 };
      out.keys.push("autoscaling");
    }
  }
  for (const k of SIZE_KEYS) {
    if (attrs[k]) {
      out.size = attrs[k].trim().split(/[\s,]/)[0];
      out.keys.push(k);
      break;
    }
  }
  return out;
}

/** The diagram attributes that describe a Sizing (what Apply writes back). */
export function sizingAttrs(s: { count: number; size?: string; scale?: { min: number; max: number; target: number } }) {
  const a: Record<string, string> = { instances: String(s.count) };
  if (s.scale) a.autoscaling = `${s.scale.min}-${s.scale.max}, target ${Math.round(s.scale.target * 100)}% requests`;
  if (s.size) a.instance = s.size;
  return a;
}

// Kinds that split incoming traffic across their outgoing Edges instead of
// calling every one of them (a CDN or load balancer routes each request once).
const ROUTERS = new Set(["network.cdn", "network.load-balancer", "network.api-gateway", "network.internet-gateway", "network.transit"]);
// Kinds a request passes through on its way in (inspection, not a destination).
const INLINE = new Set(["security.waf", "security.firewall"]);
// Outbound paths: not per request unless the user says so.
const EGRESS = new Set(["network.nat", "network.internet-gateway"]);
const COMPUTE = /^compute\./;

/** Which Profile a Node simulates as: an explicit override, or a self-hosted service on compute. */
function simKind(n: GNode, profiles: Profiles, ov?: SizingOverride): { kind: string; why?: string; service?: string } {
  if (ov?.simAs) return { kind: ov.simAs, why: "set in the Scenario" };
  if (n.attrs?.simulate_as) return { kind: n.attrs.simulate_as, why: "simulate_as on the diagram" };
  const kind = n.kind ?? "";
  const p = profiles[kind];
  const selfHosted = /fargate|ecs|eks|kubernetes|ec2|container|self[- ]hosted/i.test(n.serviceName ?? "") || COUNT_KEYS.some((k) => n.attrs?.[k]);
  if (p?.capacity?.model === "hop" && selfHosted && !n.service) {
    // Priced like the platform it runs on, when the name says which.
    const service = /fargate/i.test(n.serviceName ?? "") ? "aws.ecs-fargate" : undefined;
    return { kind: "compute.container", service, why: `${n.label} runs on your own compute (${n.serviceName ?? "tasks"}), so it is simulated as a container` };
  }
  return { kind };
}

function profileFor(profiles: Profiles, kind: string, service?: string) {
  const p = profiles[kind];
  if (!p) return null;
  let svc = (service && p.services?.[service]) || {};
  if (svc.inherit) svc = { ...p.services[svc.inherit], ...svc };
  return { p, svc, capacity: svc.capacity ?? p.capacity ?? {}, latency: svc.latency ?? p.latency ?? {}, sizes: (svc.sizes ?? p.sizes ?? []) as SizeOpt[] };
}

/** A read/write mix of a per-op value, by the Brief's read share. */
const mix = (byOp: any, read: number) => read * T(byOp.read) + (1 - read) * T(byOp.write);

export function resolve(g: Graph, profiles: Profiles, answers: Record<string, string>, sc: Scenario): Resolved {
  const bd = briefDefaults(answers);
  const notes: Note[] = [];
  const excluded: Resolved["excluded"] = [];
  const sizing: Record<string, Sizing> = {};
  const nodes: NodeSpec[] = [];
  const ids: string[] = [];
  const idx = new Map<string, number>();
  let costExcludes = 0;

  type In = { id: string; label: string; kind?: string; service?: string; attrs?: Record<string, string>; serviceName?: string; added?: boolean };
  const all: In[] = [...g.nodes, ...sc.added.map((a) => ({ id: a.ref, label: a.label, kind: "data.cache", service: cacheService(g, profiles), added: true }))];

  for (const n of all) {
    const ov = sc.sizing[n.id];
    const isSource = (n.kind ?? "").startsWith("external.") && n.kind !== "external.service";
    if (isSource) {
      // Users and clients make the traffic: an instant, unlimited pass-through.
      idx.set(n.id, nodes.length);
      ids.push(n.id);
      nodes.push({ id: n.id, label: n.label, kind: n.kind ?? "external.user", count: 1, capPerInstance: 1e9, medianMs: 0.01, sigma: 0.1, costPerHour: 0, x: 0, y: 0, w: 0, h: 0 });
      continue;
    }
    const { kind, why, service: platform } = simKind(n as GNode, profiles, ov);
    if (why && !ov?.simAs) notes.push({ id: n.id, text: why });
    const pr = profileFor(profiles, kind, kind === n.kind ? n.service : platform);
    if (!pr) {
      excluded.push({ id: n.id, label: n.label, why: n.kind ? `not on the request path (${n.kind})` : "no Component Kind" });
      continue;
    }
    const { p, svc, capacity: c, latency: lat, sizes } = pr;
    const fromAttrs = sizingFromAttrs(n.attrs);
    const defaults = p.defaults ?? {};
    let count = ov?.count ?? fromAttrs.count ?? defaults.count ?? 1;
    let size = ov?.size ?? fromAttrs.size ?? defaults.size;
    let sizeOpt = sizes.find((s) => s.id === size);
    const named = ov?.size ?? fromAttrs.size; // a size the user or the diagram chose (not a Profile default)
    if (size && !sizeOpt && sizes.length && !named) {
      // A generic default on a Service with its own sizes: the nearest by vCPU, then memory.
      const want = (p.sizes ?? []).find((s: SizeOpt) => s.id === size);
      sizeOpt = [...sizes].sort((x, y) => Math.abs((x.vcpu ?? 0) - (want?.vcpu ?? 0)) - Math.abs((y.vcpu ?? 0) - (want?.vcpu ?? 0)) || Math.abs((x.ram_gb ?? 0) - (want?.ram_gb ?? 0)) - Math.abs((y.ram_gb ?? 0) - (want?.ram_gb ?? 0)))[0];
      size = sizeOpt.id;
    }
    if (size && !sizeOpt && sizes.length) {
      notes.push({ id: n.id, text: `${n.label}: size “${size}” isn't in the ${kind} Profile; simulated as ${sizes[0].id}` });
      sizeOpt = sizes[0];
      size = sizeOpt.id;
    }
    sizeOpt ??= sizes.find((s) => s.id === defaults.size) ?? sizes[0];
    const scale = ov && "scale" in ov ? ov.scale ?? undefined : fromAttrs.scale ?? (fromAttrs.count == null ? defaults.scale : undefined);
    const readShare = bd.readShare;

    let cap = 1e6;
    let maxWaitS = 5;
    let cost = 0;
    let priced = false;
    switch (c.model) {
      case "cpu": {
        const cpu = c.cpu_ms_by_op ? mix(c.cpu_ms_by_op, readShare) : T(c.cpu_ms);
        cap = ((sizeOpt?.vcpu ?? 1) * 1000 * (c.max_util ?? 1)) / cpu;
        break;
      }
      case "rate":
        cap = T(c.ops_per_node);
        break;
      case "quota":
        cap = c.per_unit ? mix({ read: c.per_unit.reads, write: c.per_unit.writes ?? c.per_unit.reads }, readShare) * (ov?.count ?? T(c.units)) : T(c.rate_per_s);
        count = 1;
        maxWaitS = 0.05; // quotas reject at once (ticket 002)
        break;
      case "concurrency":
        cap = 1000 / T(lat.service_ms);
        count = ov?.count ?? fromAttrs.count ?? T(c.account_concurrency);
        maxWaitS = 0.05;
        break;
      case "hop":
        cap = c.rate_per_s ? T(c.rate_per_s) : 1e6;
        count = 1;
        maxWaitS = 0.05;
        break;
      default: // managed, unbounded
        count = 1;
    }
    // Hourly price: a priced size, per-vCPU/GB rates, or a flat hourly charge.
    const price = svc.price ?? null;
    if (sizeOpt?.price_hour != null) {
      cost = sizeOpt.price_hour;
      priced = true;
    } else if (price?.vcpu_hour != null && sizeOpt) {
      cost = price.vcpu_hour * (sizeOpt.vcpu ?? 1) + (price.gb_hour ?? 0) * (sizeOpt.ram_gb ?? 0);
      priced = true;
    } else if (price?.per_hour != null) {
      cost = price.per_hour;
      priced = true;
    }
    if (!priced && !["unbounded"].includes(c.model)) costExcludes++;

    const service = typeof lat.service_ms_by_op === "object" ? mix(lat.service_ms_by_op, readShare) : T(lat.service_ms);
    const warm = T(p.scaling?.warmup_s) || 45;
    idx.set(n.id, nodes.length);
    ids.push(n.id);
    nodes.push({
      id: n.id, label: n.label, kind, count, capPerInstance: cap, medianMs: service, sigma: lat.spread ?? 0.5,
      costPerHour: priced && !["quota", "hop", "managed"].includes(c.model) ? cost : 0,
      buffered: kind === "messaging.queue", maxWaitS,
      scale: scale ? { ...scale, warmupS: warm, cooldownS: 300, metric: "requests" } : undefined,
      x: 0, y: 0, w: 0, h: 0,
    });
    // Flat hourly charges (NAT, load balancers) count once, not per instance.
    if (priced && ["quota", "hop", "managed"].includes(c.model)) nodes[nodes.length - 1].costPerHour = cost;
    sizing[n.id] = { kind: n.kind ?? kind, simAs: kind !== n.kind ? kind : undefined, model: c.model, count, size: sizeOpt?.id, sizes, scale, fromDiagram: fromAttrs.keys, capPerInstance: cap };
  }

  // ---- Edges: drop those touching excluded Nodes, then work out calls per request.
  type E = { id: string; from: string; to: string; style?: string };
  // Watchers (observe) carry no requests.
  const raw: E[] = g.edges.filter((e) => e.style !== "observe" && idx.has(e.from) && idx.has(e.to));
  for (const a of sc.added) raw.push({ id: `${a.ref}:in`, from: a.from, to: a.ref, style: "sync" });
  const kindOf = (id: string) => nodes[idx.get(id)!].kind;
  const outs = new Map<string, E[]>();
  for (const e of raw) (outs.get(e.from) ?? outs.set(e.from, []).get(e.from)!).push(e);

  const edges: EdgeSpec[] = [];
  const edgeIds: string[] = [];
  for (const [from, es] of outs) {
    const fk = kindOf(from);
    const routed = es.filter((e) => !INLINE.has(kindOf(e.to)) && !(COMPUTE.test(fk) && EGRESS.has(kindOf(e.to))));
    const split = ROUTERS.has(fk) && routed.length > 1;
    if (split) notes.push({ id: from, text: `${nodes[idx.get(from)!].label} splits traffic evenly across ${routed.length} routes; set the shares on its Edges` });
    for (const e of es) {
      const tk = kindOf(e.to);
      let mult = 1;
      if (COMPUTE.test(fk) && EGRESS.has(tk)) mult = 0; // outbound calls aren't per request
      else if (split && routed.includes(e)) mult = 1 / routed.length;
      if (fk === "network.cdn" && tk === "data.object") mult *= 0.3; // the CDN answers 70% of static requests
      // A cache in front of a store: the store only sees the misses.
      const cache = sc.added.find((a) => a.from === e.from && a.to === e.to);
      if (cache) mult *= 1 - cache.hit;
      const fo = sc.flow[e.id] ?? {};
      edges.push({
        id: e.id, from: idx.get(e.from)!, to: idx.get(e.to)!, mult: fo.mult ?? mult, async: e.style === "async" || e.style === "stream",
        par: false, timeoutMs: fo.timeoutMs ?? (tk.startsWith("security.") ? 1000 : 2000), retries: fo.retries ?? 1,
      });
      edgeIds.push(e.id);
    }
  }
  if (g.nodes.some((n) => sizing[n.id] && COMPUTE.test(sizing[n.id].simAs ?? n.kind ?? "")) && raw.some((e) => EGRESS.has(kindOf(e.to)) && COMPUTE.test(kindOf(e.from)))) {
    notes.push({ text: "Outbound calls through NAT are not counted per request; set a rate on those Edges if your code calls out on every request" });
  }

  // ---- Cycles: the engine needs a call tree. Drop back edges (found by DFS).
  const state = new Map<number, number>();
  const adj = new Map<number, number[]>();
  edges.forEach((e, i) => (adj.get(e.from) ?? adj.set(e.from, []).get(e.from)!).push(i));
  const dropped = new Set<number>();
  const dfs = (v: number) => {
    state.set(v, 1);
    for (const i of adj.get(v) ?? []) {
      const w = edges[i].to;
      if (state.get(w) === 1) dropped.add(i);
      else if (!state.get(w)) dfs(w);
    }
    state.set(v, 2);
  };
  nodes.forEach((_, v) => !state.get(v) && dfs(v));
  // A loop through an Edge carrying no traffic (outbound via NAT) changes nothing: drop it quietly.
  for (const i of dropped) if (edges[i].mult > 0) notes.push({ id: edgeIds[i], text: `${nodes[edges[i].from].label} → ${nodes[edges[i].to].label} closes a loop; left out of the simulation` });

  // ---- Routes: traffic enters at sources nothing calls (users, clients), split evenly.
  const incoming = new Set(edges.filter((_, i) => !dropped.has(i)).map((e) => e.to));
  let entries = nodes.map((n, i) => ({ n, i })).filter(({ n, i }) => !incoming.has(i) && n.kind.startsWith("external."));
  if (!entries.length) {
    entries = nodes.map((n, i) => ({ n, i })).filter(({ i }) => !incoming.has(i) && edges.some((e) => e.from === i));
    notes.push({ text: "No user or client on the diagram: traffic enters at the Nodes nothing calls" });
  }
  const routes: Route[] = entries.map(({ n, i }) => ({ name: n.label, entry: i, share: 1 / entries.length, sloMs: bd.sloMs }));
  if (bd.trafficAssumed && !sc.trafficRps) notes.push({ text: "The Brief has no throughput; traffic starts at 100 rps" });

  return {
    system: { nodes, edges: edges.filter((_, i) => !dropped.has(i)), routes },
    ids, edgeIds: edgeIds.filter((_, i) => !dropped.has(i)), sizing, excluded, notes, costExcludes,
    sloMs: bd.sloMs, trafficRps: sc.trafficRps || bd.trafficRps, cacheService: cacheService(g, profiles),
  };
}

/** The cache Service for the target cloud: Valkey on AWS, Managed Redis on Azure. */
function cacheService(g: Graph, profiles: Profiles): string | undefined {
  const aws = g.nodes.some((n) => n.service?.startsWith("aws."));
  const azure = g.nodes.some((n) => n.service?.startsWith("azure."));
  const svcs = profiles["data.cache"]?.services ?? {};
  if (aws && svcs["aws.elasticache-valkey"]) return "aws.elasticache-valkey";
  if (azure && svcs["azure.managed-redis"]) return "azure.managed-redis";
  return undefined;
}
