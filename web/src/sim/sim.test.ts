// Behaviour tests for the sim: run with `npm test` (node --test, no extra dependencies).
import { test } from "node:test";
import assert from "node:assert/strict";
import type { Graph } from "../types.ts";
import { Engine, TICK_S } from "./engine.ts";
import { newScenario, resolve, sizingFromAttrs, type Profiles } from "./resolve.ts";
import { scenarioChanges } from "./apply.ts";
import { runSummary } from "./run.ts";
import type { System } from "./system.ts";

const P: Profiles = {
  "network.load-balancer": { capacity: { model: "managed" }, latency: { service_ms: { low: 1, typical: 1, high: 2 }, spread: 0.3 } },
  "network.cdn": { capacity: { model: "unbounded" }, latency: { service_ms: { low: 5, typical: 10, high: 20 }, spread: 0.3 } },
  "compute.container": {
    capacity: { model: "cpu", cpu_ms: { low: 1, typical: 4, high: 8 }, max_util: 0.9 }, latency: { service_ms: { low: 4, typical: 8, high: 20 }, spread: 0.5 },
    scaling: { warmup_s: { low: 30, typical: 45, high: 60 } }, defaults: { count: 2, size: "1vcpu-2gb" },
    sizes: [{ id: "1vcpu-2gb", vcpu: 1, ram_gb: 2 }, { id: "2vcpu-4gb", vcpu: 2, ram_gb: 4 }],
    services: { "aws.ecs-fargate": { price: { vcpu_hour: 0.04, gb_hour: 0.004 } } },
  },
  "data.relational": {
    capacity: { model: "cpu", cpu_ms: { low: 0.1, typical: 0.4, high: 2 }, max_util: 0.8 }, latency: { service_ms: { low: 1, typical: 2, high: 5 }, spread: 0.7 },
    defaults: { count: 1, size: "2vcpu-16gb" }, sizes: [{ id: "2vcpu-16gb", vcpu: 2, ram_gb: 16 }],
    services: { "aws.rds-postgres": { sizes: [{ id: "db.t4g.medium", vcpu: 2, ram_gb: 4, price_hour: 0.065 }, { id: "db.r6g.large", vcpu: 2, ram_gb: 16, price_hour: 0.225 }] } },
  },
  "data.cache": { capacity: { model: "rate", ops_per_node: { low: 4e4, typical: 8e4, high: 2e5 } }, latency: { service_ms: { low: 0.2, typical: 0.5, high: 1 }, spread: 0.4 }, services: { "aws.elasticache-valkey": {} } },
  "security.identity": { capacity: { model: "hop", rate_per_s: { low: 50, typical: 120, high: 1000 } }, latency: { service_ms: { low: 30, typical: 80, high: 300 }, spread: 0.5 } },
  "network.nat": { capacity: { model: "hop" }, latency: { service_ms: { low: 0.1, typical: 0.3, high: 1 }, spread: 0.3 }, services: { "aws.nat-gateway": { price: { per_hour: 0.045 } } } },
};

// User → CDN → {static bucket… (excluded: no Profile), ALB → API → DB, API → Auth (self-hosted), API → NAT, API → Secrets}
const G: Graph = {
  production: true, groups: [], notes: null, issues: null,
  nodes: [
    { id: "user", label: "User", kind: "external.user" },
    { id: "cdn", label: "CDN", kind: "network.cdn", service: "aws.cloudfront" },
    { id: "alb", label: "ALB", kind: "network.load-balancer", service: "aws.alb" },
    { id: "api", label: "API", kind: "compute.container", service: "aws.ecs-fargate", attrs: { tasks: "2-4 across 2 AZs" } },
    { id: "auth", label: "Auth", kind: "security.identity", serviceName: "Keycloak (on ECS Fargate)", attrs: { tasks: "2" } },
    { id: "db", label: "DB", kind: "data.relational", service: "aws.rds-postgres", attrs: { instance: "db.t4g.medium", multi_az: "true" }, parent: "subnet" },
    { id: "nat", label: "NAT", kind: "network.nat", service: "aws.nat-gateway" },
    { id: "sec", label: "Secrets", kind: "security.secrets", service: "aws.secrets-manager" },
  ],
  edges: [
    { id: "e1", from: "user", to: "cdn", style: "sync" },
    { id: "e2", from: "cdn", to: "alb", style: "sync" },
    { id: "e3", from: "alb", to: "api", style: "sync" },
    { id: "e4", from: "api", to: "db", style: "sync" },
    { id: "e5", from: "api", to: "auth", style: "sync" },
    { id: "e6", from: "api", to: "nat", style: "sync" },
    { id: "e7", from: "api", to: "sec", style: "sync" },
    { id: "e8", from: "db", to: "api", style: "sync" }, // a loop, to be dropped
  ],
};
const BRIEF = { throughput: "100-1k", latency: "lt-300ms", read_write_bias: "read-heavy" };

test("sizing is read from free-text attributes", () => {
  assert.deepEqual(sizingFromAttrs({ tasks: "2-4 across 2 AZs" }), { count: 2, scale: { min: 2, max: 4, target: 0.6 }, keys: ["tasks"] });
  assert.equal(sizingFromAttrs({ instance: "db.t4g.medium" }).size, "db.t4g.medium");
  assert.deepEqual(sizingFromAttrs({ instances: "4", autoscaling: "4-10, target 70% requests" }).scale, { min: 4, max: 10, target: 0.7 });
});

test("the diagram resolves into a simulated system", () => {
  const r = resolve(G, P, BRIEF, newScenario(1, BRIEF));
  const ids = r.system.nodes.map((n) => n.id);
  assert.ok(!ids.includes("sec"), "off-path Secrets Manager is left out");
  assert.deepEqual(r.excluded.map((x) => x.id), ["sec"]);
  assert.equal(r.trafficRps, 500);
  assert.equal(r.sloMs, 300);
  // Self-hosted Keycloak simulates as a container with its own tasks.
  const auth = r.system.nodes[ids.indexOf("auth")];
  assert.equal(auth.kind, "compute.container");
  assert.equal(auth.count, 2);
  // The API's tasks and autoscaling come from "2-4".
  const api = r.system.nodes[ids.indexOf("api")];
  assert.equal(api.count, 2);
  assert.deepEqual([api.scale?.min, api.scale?.max], [2, 4]);
  // db.t4g.medium is a known RDS size: 2 vCPU × 1000 / 0.4 ms × 0.8.
  assert.equal(r.system.nodes[ids.indexOf("db")].capPerInstance, 4000);
  // Outbound through NAT isn't per request; the loop db → api is dropped.
  const nat = r.system.edges.find((e) => r.system.nodes[e.to].id === "nat")!;
  assert.equal(nat.mult, 0);
  assert.ok(!r.system.edges.some((e) => r.system.nodes[e.from].id === "db"));
  assert.equal(r.system.routes.length, 1);
  assert.equal(r.system.nodes[r.system.routes[0].entry].id, "user");
});

test("a baseline run finds the breaking point, and fixing the bottleneck moves it", () => {
  const sc = newScenario(1, BRIEF);
  const r = resolve(G, P, BRIEF, sc);
  const base = runSummary(r.system, { seed: 1, trafficRps: r.trafficRps, sloMs: r.sloMs, costExcludes: r.costExcludes });
  assert.ok(base.breakRps != null, "it breaks somewhere");
  // Every API call also calls Auth, which has a fixed 2 tasks while the API scales to 4.
  assert.equal(base.busiest[0].id, "auth", `Auth is the bottleneck, got ${JSON.stringify(base.busiest)}`);
  // Same seed, same summary.
  const again = runSummary(r.system, { seed: 1, trafficRps: r.trafficRps, sloMs: r.sloMs, costExcludes: r.costExcludes });
  assert.equal(again.breakRps, base.breakRps);
  sc.sizing.api = { count: 6, scale: { min: 6, max: 20, target: 0.6 } };
  sc.sizing.auth = { count: 6 };
  const r2 = resolve(G, P, BRIEF, sc);
  const test2 = runSummary(r2.system, { seed: 1, trafficRps: r2.trafficRps, sloMs: r2.sloMs, costExcludes: r2.costExcludes });
  assert.ok(test2.breakRps! > base.breakRps!, `${test2.breakRps} > ${base.breakRps}`);
  assert.ok(test2.costPerHour > base.costPerHour);
});

test("Scenario changes become a Proposal", () => {
  const sc = newScenario(1, BRIEF);
  sc.sizing.api = { count: 4, scale: { min: 4, max: 10, target: 0.6 } };
  sc.sizing.db = { size: "db.r6g.large" };
  sc.added.push({ ref: "+cache", label: "Cache", from: "api", to: "db", hit: 0.9 });
  const r = resolve(G, P, BRIEF, sc);
  const { changes } = scenarioChanges(G, sc, r);
  const api = changes.find((c) => c.ref === "api")!;
  assert.deepEqual(api.set, { instances: "4", autoscaling: "4-10, target 60% requests", tasks: null });
  assert.equal(changes.find((c) => c.ref === "db")!.set!.instance, "db.r6g.large");
  const add = changes.find((c) => c.op === "add_node")!;
  assert.equal(add.service, "aws.elasticache-valkey");
  assert.equal(add.parent, "subnet");
  assert.ok(changes.some((c) => c.op === "add_edge" && c.from === "api" && c.to === "+cache"));
  // The store only sees the cache's misses.
  const toDb = r.system.edges.find((e) => r.system.nodes[e.from].id === "api" && r.system.nodes[e.to].id === "db")!;
  assert.ok(Math.abs(toDb.mult - 0.1) < 1e-9);
});

test("the engine matches M/M/c waiting below saturation", () => {
  // One server, 80% busy: mean wait ≈ ρ/(μ−λ) = 0.8 / (100 − 80) s = 40 ms.
  const sys: System = {
    nodes: [{ id: "s", label: "s", kind: "compute.container", count: 1, capPerInstance: 100, medianMs: 0.01, sigma: 0.01, costPerHour: 0, x: 0, y: 0, w: 0, h: 0 }],
    edges: [], routes: [{ name: "r", entry: 0, share: 1, sloMs: 300 }],
  };
  const e = new Engine(sys, 3);
  e.traffic = 80;
  for (let i = 0; i < 10 / TICK_S; i++) e.step();
  const wait = e.pwait[0] * e.wqMs[0];
  assert.ok(Math.abs(wait - 40) < 2, `mean wait ${wait}`);
});
