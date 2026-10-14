---
id: "002"
title: "What is the sim engine model?"
labels: [wayfinder:grilling]
status: closed
assignee: claude (recommended options, for vinu's review)
blocked_by: []
parent: map
---

# What is the sim engine model?

## Question

What exactly does the engine compute each tick, and with which formulas? It must be simple enough to explain in a tooltip and believable enough that the breaking points are right in order and roughly right in size.

- **State per Node:** instances (current, warming up, draining), capacity, queue length, utilisation, error rate, current latency distribution. Per Edge: flow rate, in-flight requests, errors.
- **Capacity and queueing:** how arrival rate, per-instance capacity and instance count give utilisation and queueing delay (M/M/c or a simpler approximation), and what happens past saturation (queue grows, then timeouts, then errors, then retries make it worse).
- **Limits other than CPU:** memory (a working set larger than RAM slows down, then fails), connection pools, database connections, disk or IOPS. Which ones are in v1.
- **Dynamics:** autoscaling (target, delay, warm-up, cooldown, min/max), cold starts for serverless, a cache warming up, a queue absorbing a spike and draining later.
- **Failures:** a Node or zone going down, health checks and load balancer failover, retry storms, circuit breakers (in or out?).
- **Latency distributions for probes:** how a Node's delay is drawn (base latency distribution plus queueing wait), how fan-out (parallel calls wait for the slowest) and sequential calls compose, how timeouts and retries show up.
- **Determinism:** seeded random generator, fixed tick, so the same Scenario, seed and edit log replays exactly.
- **Where it runs:** TypeScript in the browser worker only, or also in Go for tests, the Assistant and documents? (One implementation is easier to keep right.)

## Resolution

Decided 2026-10-15, taking the recommended option throughout (vinu: "assume best practices as the preferred option"). The spike's `spikes/sim-001/src/engine.ts` is the reference shape and has been measured (ticket 001).

**Tick.** The tick is fixed at 50 ms of simulated time. Traffic enters at Route entries and flows through the graph in topological order. Each Node:
- serves up to its capacity (instances × per-instance rate, taken from the Behaviour Profile's capacity model);
- holds the rest in a backlog;
- sheds requests that have waited longer than its **shed time**.

**Capacity models** (ticket 004) turn a Profile plus Sizing into a capacity:

| Model | Capacity | Over capacity |
|---|---|---|
| `cpu` | vCPU × 1000 / cpu_ms × max_util per instance | backlog, then shed |
| `rate` | ops per node × nodes | backlog, then shed |
| `quota` | the provider's limit (per partition, prefix or account) | **rejected at once** (429/503, no queue) |
| `concurrency` | concurrency / duration | rejected (429) once concurrency and the scaling rate are used up |
| `managed` | grows by `ramp_factor_per_5min` from the recent peak | errors while it scales |
| `unbounded` | none | none |
| `hop` | none of its own: adds latency on the request path, plus any published quota (rate, connections, bandwidth) | rejected at once when a quota is exceeded (added with ticket 011's second batch for gateways, endpoints, WAF, firewalls, identity and workflows) |

**Queueing.**
- **Below saturation:** the probability of waiting and the mean wait come from Erlang C (M/M/c, c = ready instances). It's one cheap formula and gets the shape of the curve right: latency climbs sharply as utilisation approaches 100%.
- **At or past saturation:** the backlog grows as (arrivals − capacity) × time, and the wait is backlog / capacity.
- **Shed time:** a non-buffered Node's default shed time is its callers' longest timeout, since nobody is waiting for a request older than that (ticket 001, finding 5). Queues and their consumers are buffered: their backlog waits, and **backlog age** is reported instead.

**Retries and timeouts.** An Edge's flow is base flow × (1 + f + f² + … up to its retry count), where f is the timeout-and-error share the probes measured on that Edge over the last second. This produces retry storms on its own: a slow Node raises f, more retries arrive, and it gets slower.

**Limits in v1:**
- CPU or rate, quotas and concurrency.
- **Database connections:** `max_per_gb_ram` × RAM against callers' pool sizes × instances; when it's exceeded, new instances' calls fail.
- **Memory:**
  - containers: base + in-flight requests × per-request memory; over RAM, the instance restarts and is replaced after `replace_s`;
  - caches: evictions lower the hit ratio when the working set doesn't fit;
  - relational stores: a working set larger than RAM multiplies CPU and latency by the Profile's factors.

Disk and IOPS are left for later; managed stores fold them into their CPU and quota numbers.

**Dynamics:**
- **Autoscaling** is target tracking on smoothed utilisation (30 s), evaluated every 15 s, scaling out to `ceil(count × util / target)`. New instances are ready after the Profile's `warmup_s`; scale-in waits for a 300 s cooldown.
- **Serverless** gains up to `scale_rate_per_10s` environments, and each new environment's first request pays `cold_start_ms`. Environments idle for `idle_reap_s` go away.
- **Caches** start empty in a new Scenario: the hit ratio rises to its target over a warm-up period (default 5 simulated minutes, or warm at start if the user picks "start warm").
- **Queues** absorb spikes and drain at the consumers' capacity.

**Failures (v1):**
- **Kill or restore** a Node: zero instances, so calls fail fast.
- **Zone down:** Nodes inside a zone Group lose their instances in that zone. A Node spread over n zones (Sizing `zones`, default 2) loses 1/n.
- **Load balancer health checks** stop sending traffic to a failed instance after a detection time (default 10 s), so for that long a share of calls fails.
- **Database failover:** a primary failure makes the store unavailable for the Profile's `failover_s`, then the standby takes over.
- **Slow dependency:** multiply a Node's service time.
- **Circuit breakers** are in, as an Edge option. A breaker opens when errors exceed 50% over 10 s with at least 20 calls, fails fast for 30 s, then lets traffic through half-open. They're cheap to compute, and showing one stop a retry storm is one of the most useful lessons Sim Mode can teach.

**Probes** are sampled requests walked through the graph:
- At each Node, the delay is the backlog wait, plus the Erlang C wait (with that probability), plus a service time drawn from the Profile's log-normal (median and spread).
- Calls on sequential Edges add up; calls marked parallel wait for the slowest.
- A call slower than its Edge's timeout costs the timeout and is retried.
- Async Edges add no latency for the caller.

Probe rules:
- **Stratified per Route**, weighted back to traffic share.
- **Budget:** 100k probes per real second, floor 1k per simulated second, depth cap 16, 400 visits per probe.
- **Latency storage:** log-bucketed histograms (±1%) (ticket 007).

**Cycles** (A calls B, B calls back A) are allowed. The fluid pass condenses strongly connected parts and iterates each up to 5 times per tick, starting from the previous tick's flows. The probe depth cap stops endless recursion.

**Determinism.**
- A seeded generator (mulberry32 from the Scenario's seed); `Math.random` is never used.
- Edits and events apply at tick boundaries and are logged with their tick number.
- The same Scenario, seed and edit log replay exactly.

**One implementation, in TypeScript.** The engine lives in `web/src/sim/` and runs in the worker. Go never simulates: what the Assistant and documents need is a run summary the browser computes and saves (ticket 010).

**Tests** are behavioural golden tests on small graphs, run with Node's built-in test runner on the TypeScript directly (`node --test web/src/sim/`), so no new dependencies. Cases:
- one server below and past saturation (matches M/M/c within 5%);
- a cache in front of a database;
- a spike into a queue that drains;
- a slow dependency with retries collapsing;
- a breaker stopping that collapse;
- replay determinism.

`make check` runs them.
