---
id: "011"
title: "Write the starter Behaviour Profiles"
labels: [wayfinder:task]
status: closed
assignee: claude
blocked_by: ["004"]
parent: map
---

# Write the starter Behaviour Profiles

## Question

The format from ticket 004 needs proving on real content, as the pilot Knowledge Base Records did for theirs (v1 map, ticket 008). Write Profiles for a starter set of about 10 Component Kinds that cover a typical web system, with AWS and Azure Service overrides for the most common ones:

- load balancer, API gateway, CDN
- container service, serverless function
- relational database, key-value cache, document database
- queue, object storage

Then check them on three hand-built example systems (a CRUD web app, a read-heavy catalog with a cache, a write-heavy event pipeline):

- Do they produce a believable bottleneck order?
- Do the numbers land within plausible ranges of published experience?
- Does the format hold, or does it need changes?

All numbers are our own estimates with sources and dates (ADR 0002), and the copy check must pass.

Output: the Profiles, the check results, and fixes to the format.

## Resolution

Closed 2026-10-15. The Profiles are in [internal/catalog/profiles/](../../../internal/catalog/profiles/), 10 Kinds (not yet embedded):

| Group | Kinds |
|---|---|
| network | `network.load-balancer`, `network.api-gateway`, `network.cdn` |
| compute | `compute.container`, `compute.function` |
| data | `data.relational`, `data.cache`, `data.key-value`, `data.object` |
| messaging | `messaging.queue` |

`data.key-value` (DynamoDB) replaced the document database: its limits are published and well defined, which makes a better test of the `quota` model. The document database comes next.

AWS Services have prices and sizes. Azure Services are listed with `price: null` until sourced.

**Sources used** (facts only, in our own words):
- provider limits pages for S3, DynamoDB, API Gateway, SQS and Lambda scaling, plus ALB pricing;
- Redis's benchmark and pipelining docs;
- two published managed-Postgres pgbench runs;
- Fargate pricing.

Every number has a range and a `confidence`. Prices are dated, and all but Fargate's are marked `verify: true`: they come from search results and memory, not yet checked against the providers' own pricing pages.

**Check:** `spikes/sim-001/bench/profiles-check.ts` runs each system through the spike engine with typical values and ramps traffic by 5% steps. Results are in `spikes/sim-001/results/profiles-check.json`.

| System | Saturation order | First goal broken | Cost |
|---|---|---|---|
| CRUD web app: ALB → 3 × 1 vCPU containers → Postgres r6g.large (3 queries per request), 5% S3 reads | api at ≈670 rps (database at 50%) | ≈670 rps, p99 860 ms | $0.37/h |
| Read-heavy catalog: CloudFront (70% hit) → ALB → 2 × 1 vCPU containers → Redis (90% hit) → Postgres | api at ≈1,470 rps at the CDN | ≈1,470 rps, p99 374 ms | $0.53/h |
| Event pipeline: API Gateway → Lambda → SQS → Lambda → DynamoDB + S3 archive on one prefix | S3 at ≈3.4k, DynamoDB at ≈3.9k, API Gateway at ≈9.9k | ≈10.4k rps, at API Gateway | (see fix 2) |

The orders are believable and land where experience says they should:
- a few small containers handle hundreds of requests per second, while one mid-size Postgres has headroom;
- a CDN and cache multiply what the same containers can serve;
- the pipeline's first wall is the single-prefix archive write, then the table's partitions, then the account's API rate.

Costs match hand arithmetic from the Profile prices.

**Fixes to the format and engine** (folded into tickets 002, 004 and 007):

1. **Capacity needs an explicit `model`.** Six models (`cpu`, `rate`, `quota`, `concurrency`, `managed`, `unbounded`) cover all 10 Kinds, and each has its own fields.
2. **Usage-based prices must count.** The pipeline showed $0/h because Lambda, API Gateway, SQS, DynamoDB and S3 bill per request or per GB-second. The cost readout uses both hourly and usage prices.
3. **`quota` models reject, they don't queue.** API Gateway reached a 720 ms p99 with 0% errors in the check, because the spike queued its excess. The real service returns 429 at once, so the engine spec (ticket 002) rejects excess instantly for `quota` and `concurrency`. The spike engine wasn't changed.
4. **Async pipelines break silently.** S3 and DynamoDB saturated under the consumer while the ingest Route stayed green. Backlog age is now a first-class goal and dashboard metric.
5. **Partitions are the weakest number** (DynamoDB `partitions` typical 4 is a guess, because it depends on data size and history). Partitions are a visible Sizing field so users set what they know.

**Copy check:** the text is our own. When the Profiles are embedded, the Knowledge Base copy-check test gains the profiles directory (ticket 004).

## Second batch

Added 2026-10-15, after the AWS Component Library (ticket 013) needed Profiles for every Kind on a request path. There are now **31 Kinds**.

**Simulated (11):** `compute.vm`, `compute.web-app`, `compute.kubernetes`, `data.document`, `data.graph`, `data.search`, `data.time-series`, `data.file`, `messaging.topic`, `messaging.stream`, `messaging.event-bus`.
- EC2 sizes (t4g, c7g, m7g), which also serve as ECS and EKS pool hosts;
- App Runner's concurrency-based scaling and prices;
- DocumentDB, plus Cosmos DB as a request-unit quota;
- OpenSearch, from AWS's sizing example (about 20 queries/s per data-node vCPU);
- EFS IOPS and latency;
- SNS, EventBridge, Kinesis and MSK limits, plus Event Hubs throughput units.

**Pass-through (10)** use the new `hop` model (ticket 002): `network.internet-gateway`, `network.nat` (55,000 connections per destination), `network.private-endpoint`, `network.transit`, `network.hybrid-link`, `security.waf`, `security.firewall`, `security.identity`, `messaging.workflow`, and `external.service` (third-party APIs: latency plus a rate limit).

**Format changes** (folded into ticket 004):
- one quota shape, `per_unit` × `units` with a `units_name` (the starter `per_partition`/`per_prefix` were converted);
- `cpu_ms_by_op` and `service_ms_by_op` for read/write splits;
- Service-level `capacity` and `latency` replacements and `inherit`;
- `delivery` for async fan-out.

**Checks:**
- `spikes/sim-001/bench/profiles-lint.ts` (a prototype of ticket 004's test) passes: 31 Profiles and 111 ordered ranges. Every Simulated and Pass-through Component in the AWS library has a Profile.
- `profiles-check.ts` now runs six systems with typical values:

| System | First limit | Notes |
|---|---|---|
| EC2 web app: ALB → 2 × m7g.large → Postgres, 20% EFS reads | web tier at ≈816 rps (2 × 425/s) | database at 60%; $0.39/h, matching hand arithmetic |
| Search-backed catalog: 3 containers → DocumentDB, OpenSearch 3 × 2 vCPU (30% search), Neptune (20%) | OpenSearch at ≈113 searches/s; p99 target broke at ≈351 rps | OpenSearch only at 93% utilisation |
| Event fan-out: API Gateway → Lambda → EventBridge + Kinesis 4 shards + SNS; Kinesis → Lambda → Timestream | Kinesis at ≈3.9k rps; 1.5% errors at 4.5k | rejected, not queued (fix 3) |
| Earlier event pipeline (re-run) | API Gateway | now rejects with 3.8% errors instead of queueing to 720 ms (fix 3) |

**Findings:**
1. **Latency targets break before saturation.** The search system missed its p99 target with its busiest Node at 93%. The breaking-point summary must name the busiest Nodes even below 95% (ticket 008).
2. **Cost readouts can be badly incomplete.** That system showed $0.15/h because DocumentDB, OpenSearch and Neptune have no sourced prices. The cost tile must say "excludes N Services without prices" (ticket 007).
3. **Search is the expensive part** of a search-backed app, more than the document store or the containers. That matches AWS's sizing guidance, and it's the kind of thing Sim Mode should show.
