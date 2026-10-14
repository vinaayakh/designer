// Ticket 011: run the starter Behaviour Profiles on three example systems and report
// the order Nodes saturate in and where the first SLO breaks.
// Run: node bench/profiles-check.ts
import { readFileSync, writeFileSync } from "node:fs";
import { parse } from "yaml";
import { Engine, RM, SERIES_LEN, TICK_S } from "../src/engine.ts";
import type { EdgeSpec, NodeSpec, System } from "../src/system.ts";

const dir = new URL("../../../internal/catalog/profiles/", import.meta.url);
const P = (kind: string) => parse(readFileSync(new URL(`${kind}.yaml`, dir), "utf8"));
const T = (r: any) => (typeof r === "number" ? r : r.typical);

type Op = "read" | "write";
type Def = { id: string; kind: string; service?: string; size?: string; count?: number; op?: Op; fixedCap?: number; autoscale?: boolean };

/** Profile + sizing → engine NodeSpec (typical values). */
/** Profile (Kind → Service override, with inherit) + sizing → engine NodeSpec (typical values). */
function nodeFrom(d: Def, i: number): NodeSpec {
  const p = P(d.kind);
  let svc = d.service ? p.services?.[d.service] ?? {} : {};
  if (svc.inherit) svc = { ...p.services[svc.inherit], ...svc };
  const c = svc.capacity ?? p.capacity ?? {};
  const lat = svc.latency ?? p.latency;
  const op = d.op === "write" ? "write" : "read";
  const sizes = svc.sizes ?? p.sizes ?? [];
  const size = sizes.find((s: any) => s.id === d.size) ?? sizes[0];
  let cap = 1e6;
  let count = d.count ?? p.defaults?.count ?? 1;
  let cost = 0;
  switch (c.model) {
    case "cpu": {
      const cpu = c.cpu_ms_by_op ? c.cpu_ms_by_op[op] : c.cpu_ms;
      cap = (size.vcpu * 1000 * (c.max_util ?? 1)) / T(cpu);
      cost = size.price_hour ?? (svc.price?.vcpu_hour ? svc.price.vcpu_hour * size.vcpu + svc.price.gb_hour * size.ram_gb : 0);
      break;
    }
    case "rate":
      cap = T(c.ops_per_node);
      cost = size?.price_hour ?? 0;
      break;
    case "quota":
      cap = c.per_unit ? c.per_unit[op === "write" ? "writes" : "reads"] * T(c.units) : T(c.rate_per_s);
      count = 1;
      break;
    case "concurrency":
      // One "instance" = one execution environment; the concurrency limit is the instance count.
      cap = 1000 / T(lat.service_ms);
      count = d.count ?? T(c.account_concurrency);
      break;
    default: // managed, unbounded
      count = 1;
  }
  if (d.fixedCap) cap = d.fixedCap;
  const spec = p.defaults?.scale;
  return {
    id: d.id, label: d.id, kind: d.kind, count, capPerInstance: cap,
    medianMs: T(lat.service_ms_by_op ? lat.service_ms_by_op[op] : lat.service_ms), sigma: lat.spread ?? 0.5, costPerHour: cost,
    buffered: d.kind === "messaging.queue" || d.op === ("consumer" as any),
    maxWaitS: c.model === "quota" ? 0.05 : 5, // quotas reject at once (ticket 002)
    scale: d.autoscale && spec ? { min: spec.min, max: spec.max, target: spec.target, warmupS: T(p.scaling.warmup_s), cooldownS: 300, metric: spec.metric ?? "cpu" } : undefined,
    x: 0, y: 0, w: 0, h: 0,
  };
}

function system(defs: Def[], wires: [string, string, number, Partial<EdgeSpec>?][], entry: string): System {
  const nodes = defs.map(nodeFrom);
  const at = (id: string) => defs.findIndex((d) => d.id === id);
  const edges: EdgeSpec[] = wires.map(([f, t, mult, o], i) => ({ id: `e${i}`, from: at(f), to: at(t), mult, async: false, par: false, timeoutMs: 2000, retries: 1, ...o }));
  return { nodes, edges, routes: [{ name: "main", entry: at(entry), share: 1, sloMs: 300 }] };
}

const systems: Record<string, { sys: () => System; note: string }> = {
  "CRUD web app": {
    note: "ALB → 3 × 1 vCPU containers → Postgres db.r6g.large (3 queries per request); 5% of requests read S3",
    sys: () =>
      system(
        [
          { id: "alb", kind: "network.load-balancer", service: "aws.alb" },
          { id: "api", kind: "compute.container", service: "aws.ecs-fargate", size: "1vcpu-2gb", count: 3 },
          { id: "db", kind: "data.relational", service: "aws.rds-postgres", size: "db.r6g.large" },
          { id: "s3", kind: "data.object", service: "aws.s3", op: "read" },
        ],
        [["alb", "api", 1, { timeoutMs: 10000, retries: 0 }], ["api", "db", 3, { timeoutMs: 1000 }], ["api", "s3", 0.05]],
        "alb",
      ),
  },
  "Read-heavy catalog": {
    note: "CloudFront (70% hit) → ALB → 2 × 1 vCPU containers → Redis (2 reads/request, 90% hit) → Postgres on a miss",
    sys: () =>
      system(
        [
          { id: "cdn", kind: "network.cdn", service: "aws.cloudfront" },
          { id: "alb", kind: "network.load-balancer", service: "aws.alb" },
          { id: "api", kind: "compute.container", service: "aws.ecs-fargate", size: "1vcpu-2gb", count: 2 },
          { id: "cache", kind: "data.cache", service: "aws.elasticache-redis", size: "cache.r6g.large" },
          { id: "db", kind: "data.relational", service: "aws.rds-postgres", size: "db.r6g.large" },
        ],
        [["cdn", "alb", 0.3, { timeoutMs: 10000, retries: 0 }], ["alb", "api", 1, { timeoutMs: 10000, retries: 0 }], ["api", "cache", 2, { timeoutMs: 100 }], ["cache", "db", 0.1, { timeoutMs: 1000 }]],
        "cdn",
      ),
  },
  "Write-heavy event pipeline": {
    note: "API Gateway → Lambda ingest (500 concurrency) → SQS → Lambda consumer (500) → DynamoDB write + S3 archive write (one prefix)",
    sys: () =>
      system(
        [
          { id: "apigw", kind: "network.api-gateway", service: "aws.api-gateway" },
          { id: "ingest", kind: "compute.function", service: "aws.lambda", count: 500 },
          { id: "sqs", kind: "messaging.queue", service: "aws.sqs" },
          { id: "consumer", kind: "compute.function", service: "aws.lambda", count: 500, op: "consumer" as any },
          { id: "ddb", kind: "data.key-value", service: "aws.dynamodb", op: "write" },
          { id: "s3", kind: "data.object", service: "aws.s3", op: "write" },
        ],
        [["apigw", "ingest", 1, { timeoutMs: 29000, retries: 0 }], ["ingest", "sqs", 1, { timeoutMs: 1000 }], ["sqs", "consumer", 1, { async: true }], ["consumer", "ddb", 1], ["consumer", "s3", 1, { timeoutMs: 3000 }]],
        "apigw",
      ),
  },
};

// Systems using the second batch of Profiles (compute.vm, data.file, data.search, data.document,
// data.graph, data.time-series, messaging.stream / event-bus / topic).
Object.assign(systems, {
  "EC2 web app with shared files": {
    note: "ALB → EC2 Auto Scaling group 2 × m7g.large → Postgres r6g.large (3 queries per request); 20% of requests read a file from EFS",
    sys: () =>
      system(
        [
          { id: "alb", kind: "network.load-balancer", service: "aws.alb" },
          { id: "web", kind: "compute.vm", service: "aws.ec2", size: "m7g.large", count: 2 },
          { id: "db", kind: "data.relational", service: "aws.rds-postgres", size: "db.r6g.large" },
          { id: "efs", kind: "data.file", service: "aws.efs", op: "read" },
        ],
        [["alb", "web", 1, { timeoutMs: 10000, retries: 0 }], ["web", "db", 3, { timeoutMs: 1000 }], ["web", "efs", 0.2, { timeoutMs: 1000 }]],
        "alb",
      ),
  },
  "Search-backed catalog": {
    note: "ALB → 3 × 1 vCPU containers → DocumentDB (2 reads/request) + OpenSearch 3 × 2 vCPU (30% of requests search) + Neptune (20% get recommendations)",
    sys: () =>
      system(
        [
          { id: "alb", kind: "network.load-balancer", service: "aws.alb" },
          { id: "api", kind: "compute.container", service: "aws.ecs-fargate", size: "1vcpu-2gb", count: 3 },
          { id: "docdb", kind: "data.document", service: "aws.documentdb", size: "2vcpu-16gb" },
          { id: "search", kind: "data.search", service: "aws.opensearch", size: "2vcpu-16gb", count: 3 },
          { id: "graph", kind: "data.graph", service: "aws.neptune", size: "2vcpu-16gb" },
        ],
        [["alb", "api", 1, { timeoutMs: 10000, retries: 0 }], ["api", "docdb", 2, { timeoutMs: 1000 }], ["api", "search", 0.3, { timeoutMs: 2000 }], ["api", "graph", 0.2, { timeoutMs: 1000 }]],
        "alb",
      ),
  },
  "Event fan-out": {
    note: "API Gateway → Lambda (500) → EventBridge put + Kinesis 4 shards + SNS publish (50%); Kinesis → consumer Lambda → Timestream writes",
    sys: () =>
      system(
        [
          { id: "apigw", kind: "network.api-gateway", service: "aws.api-gateway" },
          { id: "ingest", kind: "compute.function", service: "aws.lambda", count: 500 },
          { id: "events", kind: "messaging.event-bus", service: "aws.eventbridge" },
          { id: "stream", kind: "messaging.stream", service: "aws.kinesis-data-streams", op: "write" },
          { id: "sns", kind: "messaging.topic", service: "aws.sns" },
          { id: "consumer", kind: "compute.function", service: "aws.lambda", count: 200, op: "consumer" as any },
          { id: "tsdb", kind: "data.time-series", service: "aws.timestream", op: "write" },
        ],
        [
          ["apigw", "ingest", 1, { timeoutMs: 29000, retries: 0 }], ["ingest", "events", 1, { timeoutMs: 1000 }], ["ingest", "stream", 1, { timeoutMs: 1000 }],
          ["ingest", "sns", 0.5, { timeoutMs: 1000 }], ["stream", "consumer", 1, { async: true }], ["consumer", "tsdb", 1, { timeoutMs: 1000 }],
        ],
        "apigw",
      ),
  },
});

const ticks = Math.round(1 / TICK_S);
const r = (x: number) => Math.round(x);
const out: any = {};
for (const [name, def] of Object.entries(systems)) {
  const sys = def.sys();
  const eng = new Engine(sys, 11);
  eng.probesPerSec = 4000;
  const capRps = (n: number) => sys.nodes[n].count * sys.nodes[n].capPerInstance;
  const order: string[] = [];
  const seen = new Set<number>();
  let broke = "";
  let low = "";
  const last = (m: number) => eng.routeSeries[(eng.R * RM + m) * SERIES_LEN + ((eng.seconds - 1) % SERIES_LEN)];
  for (let rps = 50; rps <= 20000 && !broke; rps = Math.ceil(rps * 1.05)) {
    eng.traffic = rps;
    for (let i = 0; i < 15 * ticks; i++) eng.step();
    if (!low) low = `p50 ${r(last(2))} ms, p99 ${r(last(4))} ms at ${rps} rps`;
    for (let n = 0; n < eng.N; n++) {
      const backlogGrowing = sys.nodes[n].buffered && eng.backlog[n] > eng.served[n] * 10; // >10 s behind
      if (!seen.has(n) && (eng.util[n] > 0.95 || backlogGrowing)) {
        seen.add(n);
        order.push(`${sys.nodes[n].id} at ${rps} rps${backlogGrowing ? " (backlog growing)" : ""}`);
      }
    }
    if (last(4) > 300 || last(1) > 0.01) broke = `${rps} rps (p99 ${r(last(4))} ms, errors ${(last(1) * 100).toFixed(1)}%)`;
  }
  out[name] = {
    system: def.note,
    capacity: Object.fromEntries(sys.nodes.map((n, i) => [n.id, `${n.count} × ${r(n.capPerInstance)}/s = ${r(capRps(i))}/s`])),
    costPerHour: Math.round(eng.costPerHour * 100) / 100,
    lowLoad: low,
    saturationOrder: order,
    firstSloBreak: broke || "none up to 20k rps",
  };
  console.log(name, JSON.stringify(out[name], null, 1));
}
writeFileSync(new URL("../results/profiles-check.json", import.meta.url), JSON.stringify(out, null, 1));
