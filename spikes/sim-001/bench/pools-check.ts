// Sim ticket 014: how an ECS service behaves under a spike on Fargate vs on an EC2 capacity pool.
// Same service (1 vCPU tasks, ≈225 rps each, target 60%, 4–40 tasks), same spike (600 → 3,000 rps).
// Run: node bench/pools-check.ts
import { writeFileSync } from "node:fs";
import { Engine, RM, SERIES_LEN, TICK_S } from "../src/engine.ts";
import type { System } from "../src/system.ts";

function system(variant: string): System {
  const metric = variant.endsWith("+requests") ? "requests" : "cpu";
  variant = variant.replace("+requests", "");
  const onPool = variant !== "fargate";
  const n = (o: any) => ({ x: 0, y: 0, w: 0, h: 0, sigma: 0.5, maxWaitS: 2, ...o });
  return {
    nodes: [
      n({ id: "alb", label: "alb", kind: "network.load-balancer", count: 1, capPerInstance: 1e6, medianMs: 1.5, costPerHour: 0.0225 }),
      n({
        id: "api", label: "api", kind: "compute.container", count: 4, capPerInstance: 225, medianMs: 8,
        // Fargate bills per task (1 vCPU + 2 GB); on EC2 the hosts carry the cost.
        costPerHour: onPool ? 0 : 0.04048 + 2 * 0.004445, vcpu: 1, pool: onPool ? 0 : undefined,
        // Task start: Fargate pulls the image onto fresh capacity (~60 s); on a warm EC2 host it's faster (~20 s).
        scale: { min: 4, max: 40, target: 0.6, warmupS: onPool ? 20 : 60, cooldownS: 300, metric },
      }),
      n({ id: "db", label: "db", kind: "data.relational", count: 1, capPerInstance: 40000, medianMs: 2, costPerHour: 0.9 }),
    ],
    edges: [
      { id: "e0", from: 0, to: 1, mult: 1, async: false, par: false, timeoutMs: 10000, retries: 0 },
      { id: "e1", from: 1, to: 2, mult: 2, async: false, par: false, timeoutMs: 1000, retries: 1 },
    ],
    routes: [{ name: "main", entry: 0, share: 1, sloMs: 300 }],
    // m7g.xlarge-like hosts: 4 vCPU. Boot + join + agent ready ≈ 150 s (estimate).
    pools: onPool ? [{ id: "pool", vcpuPerHost: 4, hosts: variant === "ec2-headroom" ? 5 : 2, scale: { min: 2, max: 12, warmupS: 150 }, costPerHostHour: 0.163 }] : undefined,
  };
}

const ticks = Math.round(1 / TICK_S);
const out: any = {};
for (const v of ["fargate", "ec2", "ec2-headroom", "fargate+requests", "ec2+requests", "ec2-headroom+requests"]) {
  const eng = new Engine(system(v), 5);
  eng.probesPerSec = 3000;
  eng.traffic = 600;
  const last = (m: number) => eng.routeSeries[(eng.R * RM + m) * SERIES_LEN + ((eng.seconds - 1) % SERIES_LEN)];
  for (let s = 0; s < 300; s++) for (let i = 0; i < ticks; i++) eng.step();
  const costBefore = eng.costPerHour;
  eng.traffic = 3000; // spike at t = 5:00
  let maxP99 = 0, maxErr = 0, badSeconds = 0, recoveredAt = -1, maxUnplaced = 0, hostsMax = 0;
  for (let s = 0; s < 900; s++) {
    for (let i = 0; i < ticks; i++) eng.step();
    const p99 = last(4), err = last(1);
    maxP99 = Math.max(maxP99, p99);
    maxErr = Math.max(maxErr, err);
    const bad = p99 > 300 || err > 0.01;
    if (bad) { badSeconds++; recoveredAt = -1; } else if (recoveredAt < 0) recoveredAt = s;
    maxUnplaced = Math.max(maxUnplaced, eng.unscheduled[1] ?? 0);
    hostsMax = Math.max(hostsMax, eng.poolHosts[0] ?? 0);
  }
  out[v] = {
    secondsOverSlo: badSeconds,
    recoveredAfterS: recoveredAt,
    worstP99Ms: Math.round(maxP99),
    worstErrPct: Math.round(maxErr * 1000) / 10,
    tasksAtEnd: eng.count[1],
    maxUnplacedTasks: maxUnplaced,
    hosts: v.startsWith("fargate") ? null : `${system(v).pools![0].hosts} → ${hostsMax}`,
    costPerHour: `${costBefore.toFixed(2)} → ${eng.costPerHour.toFixed(2)}`,
  };
  console.log(v, JSON.stringify(out[v]));
}
writeFileSync(new URL("../results/pools-check.json", import.meta.url), JSON.stringify(out, null, 1));
