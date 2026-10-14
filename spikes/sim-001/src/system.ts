// The simulated system: Nodes with sizing and a (spike-grade) Behaviour Profile,
// Edges with call behaviour, and routes (entry points with a traffic share).

export type Scale = { min: number; max: number; target: number; warmupS: number; cooldownS: number; metric?: "cpu" | "requests" };

export type NodeSpec = {
  id: string;
  label: string;
  kind: string;
  count: number; // instances
  capPerInstance: number; // requests/s one instance serves at 100% busy
  medianMs: number; // service time, log-normal
  sigma: number;
  costPerHour: number; // per instance
  buffered?: boolean; // a queue or its consumer: backlog waits instead of timing out
  maxWaitS?: number; // non-buffered: requests waiting longer than this are shed
  scale?: Scale;
  pool?: number; // index into System.pools: tasks/pods must fit on the pool's hosts (ECS on EC2, EKS node groups)
  vcpu?: number; // per instance, when placed on a pool
  x: number;
  y: number;
  w: number;
  h: number;
};

export type EdgeSpec = {
  id: string;
  from: number;
  to: number;
  mult: number; // calls per request served by `from`
  async: boolean; // through a queue: adds no latency to the caller
  par: boolean; // runs in parallel with the caller's other `par` calls
  timeoutMs: number;
  retries: number;
};

export type Route = { name: string; entry: number; share: number; sloMs: number };

/** A capacity pool: the EC2 hosts of an ECS cluster or an EKS node group. */
export type Pool = { id: string; vcpuPerHost: number; hosts: number; scale?: { min: number; max: number; warmupS: number }; costPerHostHour: number };

export type System = { nodes: NodeSpec[]; edges: EdgeSpec[]; routes: Route[]; pools?: Pool[] };

// Spike-grade defaults per role. Real numbers come from Behaviour Profiles (ticket 004).
const ROLE: Record<string, Omit<NodeSpec, "id" | "label" | "x" | "y" | "w" | "h">> = {
  lb: { kind: "network.load-balancer", count: 2, capPerInstance: 20000, medianMs: 1, sigma: 0.3, costPerHour: 0.03 },
  api: { kind: "compute.container", count: 4, capPerInstance: 400, medianMs: 8, sigma: 0.5, costPerHour: 0.1, maxWaitS: 5, scale: { min: 2, max: 20, target: 0.6, warmupS: 60, cooldownS: 300 } },
  svcA: { kind: "compute.container", count: 3, capPerInstance: 300, medianMs: 12, sigma: 0.5, costPerHour: 0.1, maxWaitS: 5, scale: { min: 2, max: 20, target: 0.6, warmupS: 60, cooldownS: 300 } },
  svcB: { kind: "compute.container", count: 2, capPerInstance: 300, medianMs: 15, sigma: 0.6, costPerHour: 0.1, maxWaitS: 5 },
  cache: { kind: "data.cache", count: 2, capPerInstance: 50000, medianMs: 0.5, sigma: 0.4, costPerHour: 0.2, maxWaitS: 1 },
  db: { kind: "data.relational", count: 1, capPerInstance: 3000, medianMs: 3, sigma: 0.7, costPerHour: 0.5, maxWaitS: 5 },
  queue: { kind: "messaging.queue", count: 1, capPerInstance: 1e6, medianMs: 5, sigma: 0.3, costPerHour: 0.01, buffered: true },
  worker: { kind: "compute.function", count: 5, capPerInstance: 40, medianMs: 30, sigma: 0.5, costPerHour: 0.05, buffered: true },
  obj: { kind: "data.object", count: 1, capPerInstance: 5500, medianMs: 20, sigma: 0.5, costPerHour: 0.02, maxWaitS: 5 },
  ext: { kind: "external.saas", count: 1, capPerInstance: 1000, medianMs: 120, sigma: 0.6, costPerHour: 0, maxWaitS: 10 },
};

const ROLES = ["lb", "api", "svcA", "svcB", "cache", "db", "queue", "worker", "obj", "ext"];
// Column and row of each role inside a domain block.
const SLOT: Record<string, [number, number]> = {
  lb: [0, 0], api: [1, 0], svcA: [2, 0], svcB: [2, 1], cache: [3, 0], db: [4, 0], queue: [3, 1], worker: [4, 1], obj: [5, 1], ext: [5, 0],
};
// [from, to, mult, async, par, timeoutMs, retries]
const WIRING: [string, string, number, boolean, boolean, number, number][] = [
  ["lb", "api", 1, false, false, 10000, 0],
  ["api", "svcA", 1, false, true, 2000, 1],
  ["api", "svcB", 0.5, false, true, 2000, 1],
  ["svcA", "cache", 1, false, false, 200, 1],
  ["cache", "db", 0.2, false, false, 1000, 0],
  ["svcB", "db", 2, false, false, 1000, 1],
  ["svcB", "queue", 0.3, true, false, 1000, 2],
  ["queue", "worker", 1, true, false, 30000, 0],
  ["worker", "db", 1, false, false, 1000, 2],
  ["worker", "obj", 1, false, false, 3000, 2],
  ["svcA", "ext", 0.1, false, false, 3000, 1],
];

export const NODE_W = 170;
export const NODE_H = 64;
const COL_W = 240;
const ROW_H = 110;
const DOMAIN_W = 6 * COL_W + 120;
const DOMAIN_H = 2 * ROW_H + 90;

/** A synthetic Production-like system of `domains` × 10 Nodes. */
export function buildSystem(domains: number): System {
  const nodes: NodeSpec[] = [];
  const edges: EdgeSpec[] = [];
  const routes: Route[] = [];
  const perRow = Math.max(1, Math.ceil(Math.sqrt(domains / 2)));
  for (let d = 0; d < domains; d++) {
    const ox = (d % perRow) * DOMAIN_W;
    const oy = Math.floor(d / perRow) * DOMAIN_H;
    const base = nodes.length;
    for (const r of ROLES) {
      const [c, row] = SLOT[r];
      const spec = ROLE[r];
      nodes.push({ ...spec, scale: spec.scale && { ...spec.scale }, id: `d${d}-${r}`, label: `${r} ${d}`, x: ox + c * COL_W, y: oy + row * ROW_H, w: NODE_W, h: NODE_H });
    }
    const at = (r: string) => base + ROLES.indexOf(r);
    for (const [f, t, mult, async, par, timeoutMs, retries] of WIRING) {
      edges.push({ id: `d${d}-${f}-${t}`, from: at(f), to: at(t), mult, async, par, timeoutMs, retries });
    }
    // A cross-domain call keeps the graph from being 50 identical islands (still acyclic: d → d+1).
    if (d + 1 < domains) edges.push({ id: `d${d}-x`, from: at("api"), to: base + 10 + ROLES.indexOf("svcA"), mult: 0.2, async: false, par: true, timeoutMs: 2000, retries: 1 });
    routes.push({ name: `route ${d}`, entry: at("lb"), share: 1 / domains, sloMs: 300 });
  }
  return { nodes, edges, routes };
}
