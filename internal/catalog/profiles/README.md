# Behaviour Profiles

The numbers Simulation runs on, one file per Component Kind, with per-Service overrides.
Format and rules: [What is a Behaviour Profile?](../../../wayfinder/sim/tickets/004-what-is-a-behaviour-profile.md).
31 Kinds: every Kind an AWS Component Library entry simulates or passes through ([ticket 011](../../../wayfinder/sim/tickets/011-write-the-starter-behaviour-profiles.md)).
They are embedded in the binary and served to Sim Mode at `GET /api/sim/profiles`. `go test ./internal/catalog` lints them: ranges ordered, prices dated, sources complete.

Capacity models: `cpu`, `rate`, `quota` (`per_unit` × `units`, or `rate_per_s`), `concurrency`, `managed`, `unbounded`
and `hop` (on the request path, adds latency and any published quota, no capacity of its own). A Service override may
replace `capacity` or `latency` wholesale (e.g. Cosmos DB's request units), or `inherit` another Service's override.
Where reads and writes differ sharply, use `cpu_ms_by_op` and `service_ms_by_op`.

- Every number is a range `{low, typical, high}`. A run uses `typical` unless its Scenario picks
  `optimistic` (the low end of cost and latency, the high end of capacity) or `pessimistic`.
- Numbers are **our own estimates**. Vendor pages, benchmarks and price lists are facts-only
  sources (ADR 0002): cite them, never copy their tables.
- `confidence` says where a number comes from: `vendor` (stated by the provider), `measured` (a published
  benchmark) or `estimate` (our judgement, with the reasoning in a comment).
- Prices are on-demand, Linux, `us-east-1` unless stated, and dated. **Prices marked `verify: true` have not
  been checked against the provider's own pricing page** and must be before release, like `hipaa_as_of`
  in the Cloud Catalogs.
