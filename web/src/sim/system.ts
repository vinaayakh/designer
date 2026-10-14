// What the engine simulates: Nodes with capacity and latency (resolved from
// Behaviour Profiles and Sizing by resolve.ts), Edges with call behaviour, and
// Routes (entry points with a share of the traffic). No Kind or cloud names
// matter here: the engine only sees numbers (sim ticket 012).

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
