---
id: "014"
title: "How do ECS and EKS simulate?"
labels: [wayfinder:grilling, wayfinder:research]
status: closed
assignee: claude (recommended options, for vinu's review)
blocked_by: ["002", "003"]
parent: map
---

# How do ECS and EKS simulate?

## Question

ECS and EKS aren't one box. An ECS service or EKS Deployment runs tasks or pods, and on EC2 those must fit on the cluster's hosts, which scale separately and take minutes to boot. On Fargate there are no hosts to manage. How is this drawn, sized and simulated, and does the difference matter enough to model?

## Resolution

Decided 2026-10-15. Measured with the spike engine (`spikes/sim-001/bench/pools-check.ts`, results in `results/pools-check.json`).

**Drawing: a cluster is a Group.**
- A new group kind, **`cluster`** (a container cluster: an ECS cluster or EKS cluster; on Azure an AKS cluster or Container Apps environment). This adds to the group kinds in [v1 ticket 017](../../tickets/017-what-are-the-component-kinds-and-cloud-catalog-contents.md).
- A cluster Group carries a `service` (`aws.ecs-fargate`, `aws.ecs-ec2` or `aws.eks`) and, when it runs on EC2, a **capacity pool**: `customData.pool` with host size, host count and a scaling rule. This is Sizing for Groups (ticket 003).
- Services and workloads are Nodes inside the cluster: an ECS service (`aws.ecs-fargate` or `aws.ecs-ec2`) or an EKS workload (`aws.eks-workload`). Their Sizing is tasks or pods and their size (vCPU, memory).

**Simulation: two-level scaling.**
- A task or pod on a pool must fit its vCPU on the pool's hosts. A new task that doesn't fit is **unplaced**: it shows on the Node as `+3 waiting for capacity`.
- The pool's scaler (ECS capacity provider or Cluster Autoscaler/Karpenter) adds hosts for the unplaced vCPU. They're ready after the host warm-up (Profile; about 150 s for EC2 to boot and join), and waiting tasks are placed as soon as room appears.
- On Fargate there's no pool. Each task starts on fresh capacity, with a longer task start (about 60 s against 20 s on a warm host) and a per-task price.
- Pool hosts cost per hour; Fargate tasks cost per vCPU and GB.

**Scaling metric.** Sizing `scale.metric` is `cpu` or `requests`:
- `cpu`: target tracking on utilisation;
- `requests`: target tracking on requests per instance, like ALBRequestCountPerTarget.

**`requests` is the default for compute behind a load balancer.** Utilisation caps at 100% under overload, so CPU tracking under-reads demand and needs several rounds to catch up.

**Measured.** One ECS service (1 vCPU tasks, ≈225 rps each, 4–40 tasks) went through a spike from 600 to 3,000 rps:

| Platform | CPU scaling: seconds over SLO | Request scaling: seconds over SLO | Cost before → after (request scaling) |
|---|---|---|---|
| Fargate | 261 | 141 | $1.17 → $2.06/h |
| EC2 pool, 2 hosts (tight) | 319 (11 tasks waited for hosts) | 252 | $1.25 → $1.90/h |
| EC2 pool, 5 hosts (headroom) | 172 | **100** | $1.74 → $1.90/h |

The trade-offs are the right ones to teach:
- Fargate recovers faster than a tight EC2 pool because nothing waits for hosts.
- An EC2 pool with headroom recovers fastest but costs more at rest.
- Scaling on requests beats scaling on CPU everywhere.

So the model is worth its cost: about 60 lines in the spike engine, with one pass over the pool's Nodes per scaling evaluation.

**Shown in Sim Mode:**
- the cluster Group's header carries a pool badge (`hosts 5 +1 · 78% vCPU`);
- unplaced tasks show on their Node;
- the dashboard's USE view gets a row per pool (vCPU used, hosts, unplaced).

The inspector explains the scaling metric with this ticket's numbers in plain words.

**EKS specifics** use the same model, plus:
- pods per node are limited by the instance's IP addresses (an estimate per host size in the Profile);
- DaemonSets reserve a share of each host (default 0.25 vCPU);
- a cluster with a Fargate profile behaves like ECS on Fargate.

Control-plane cost is a fixed hourly price on the cluster.
