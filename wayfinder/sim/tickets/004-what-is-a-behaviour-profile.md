---
id: "004"
title: "What is a Behaviour Profile?"
labels: [wayfinder:grilling, wayfinder:research]
status: closed
assignee: claude (recommended options, for vinu's review)
blocked_by: ["002", "003"]
parent: map
---

# What is a Behaviour Profile?

## Question

The engine needs numbers for every component: how much one instance of this size handles, how fast, and at what cost. These numbers decide whether the simulation is believable. What does a Behaviour Profile hold, where do its numbers come from, and how are they kept honest?

- **Fields:** capacity per instance (per vCPU, or per size), base latency distribution (median and tail), memory behaviour, connection limits, scaling behaviour (warm-up time, cold start), cache hit curve against cache size and key spread, failure modes, price per hour per size.
- **Granularity:** one Profile per Component Kind as the default, overridden per Service, overridden per size. How the layers combine.
- **Workload sensitivity:** request "weight". A heavy report query and a key lookup on the same database cost very different amounts. Is that on the Profile, on the Edge (ticket 005), or both?
- **Ranges:** every number is a range (low, typical, high). Does the user pick optimistic / typical / pessimistic for a run, and does the dashboard show a band?
- **Sources and licensing (ADR 0002):** vendor docs, public benchmarks and pricing pages are facts-only sources. Numbers must be our own estimates with a citation, never copied tables. Which sources are allowed, and how is each number's source recorded?
- **Format and location:** built-in YAML next to the Cloud Catalogs, checked against a schema at load; a user override per Project (in a plain file) for teams that know their own numbers.
- **Pricing drift:** prices change. Is the date shown, and how is it refreshed (like the catalog's `hipaa_as_of`)?

## Resolution

Decided 2026-10-15, taking the recommended option throughout. The format was proven on 10 Kinds in ticket 011 ([internal/catalog/profiles/](../../../internal/catalog/profiles/)).

**One YAML file per Component Kind** in `internal/catalog/profiles/<kind>.yaml`, embedded in the binary and checked against a schema at load, like the catalogs. Sections:

| Section | Holds |
|---|---|
| `unit` | what one call into this Kind is (request, query, operation, message, invocation) |
| `capacity` | `model` (`cpu`, `rate`, `quota`, `concurrency`, `managed`, `unbounded`, `hop`; ticket 002) and that model's numbers: `cpu_ms` (or `cpu_ms_by_op` with `read` and `write`) and `max_util`; `ops_per_node`; quotas as `per_unit` (reads and writes per unit) × `units` with a `units_name` (partitions, prefixes, shards, brokers, RU/s, file systems), or `rate_per_s`; account limits |
| `latency` | `service_ms` (median own processing time) and `spread` (log-normal sigma), plus `cold_start_ms` for functions |
| `memory` | what happens past RAM: `restart`, `evict`, or CPU and latency factors |
| `connections` | `max_per_gb_ram` for stores that limit connections |
| `scaling` | `mode` (`fixed`, `instances`, `serverless`, `managed`) and its timings (`warmup_s`, `scale_rate_per_10s`, `idle_reap_s`) |
| `failure` | `replace_s`, `failover_s` |
| `defaults` | Sizing used when a Node has none |
| `sizes` | Generic sizes |
| `services.<id>` | per-Service overrides of any of the above, plus Service `sizes` and `price`; `inherit: <service>` reuses another Service |
| `sources` | `id`, `url`, `copy` (licence tier) and `used_for` per source |

**Layers:** Kind → Service override → size. The most specific layer wins, field by field. A Service may replace `capacity` or `latency` as a whole when it works differently from its Kind's default. For example, Cosmos DB is a request-unit quota where DocumentDB is CPU-bound, and MSK counts brokers where Kinesis counts shards.

Added with ticket 011's second batch:
- `latency.service_ms_by_op`, for stores whose reads and writes differ sharply (search, time series, file systems);
- `delivery` (delay and retries), for topics and event buses whose deliveries are async;
- `capacity.max_concurrency`, for web platforms that scale on concurrent requests.

**Ranges.**
- Every estimated number is `{low, typical, high}`.
- A Scenario picks a **Profile mode**: `typical` (default), `optimistic` or `pessimistic`. Optimistic takes the favourable end of each range (low cost and latency, high capacity); pessimistic the other end.
- The dashboard shows one run's numbers, not bands: a band would mean running the engine three times. To see the spread, the user compares a typical run with a pessimistic one (ticket 007).

**Request weight lives on the Edge** (ticket 005). `weight: 3` means each call costs the target three typical requests' worth of CPU and service time. The Profile describes a typical request; the Edge describes this workload.

**Honesty rules:**
- Every number has a `confidence`: `vendor`, `measured` or `estimate`. Estimates carry a comment with the reasoning.
- Sources are facts-only (ADR 0002): numbers in our own words and units, never copied tables.
- Prices carry `region`, `as_of` and `verify: true` until checked against the provider's own pricing page, like `hipaa_as_of` in the catalogs.
- A test lints the Profiles: schema, ranges ordered low ≤ typical ≤ high, every source has a URL and licence tier, no `verify: true` in a release build (a step to add to PUBLISHING.md when the Profiles are embedded), and the copy check used for Knowledge Base Records.

**Shown to the user.**
- The inspector shows each number with its range and confidence ("≈225 req/s per instance: estimate, 66–900").
- The cost tile says "prices as of 2026-10, us-east-1".
- Usage-based prices (per request, per GB-second) count in the cost readout, not only hourly prices (ticket 011 found serverless showing $0).

**Project overrides.** A Project can override any Profile field in `sim/profiles.yaml`, using the same shape keyed by Kind or Service, for teams that have measured their own numbers. Overridden numbers are marked "Project" in the inspector.

**Refreshing.** The Profiles have a date version, like the Knowledge Base. Refreshing prices is a manual task with the same review as catalog HIPAA flags; it is listed under "Not yet specified" on the map (calibration).
