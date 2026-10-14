---
id: "003"
title: "How is a Node sized?"
labels: [wayfinder:grilling]
status: closed
assignee: claude (recommended options, for vinu's review)
blocked_by: []
parent: map
---

# How is a Node sized?

## Question

The diagram says *what* each Node is (Component Kind, Service) but not *how much* of it. What sizing does a Production Node carry, and where is it stored?

- **Fields:** instance count; size (vCPU and RAM, or a named instance type from the Cloud Catalog such as `db.r6g.large`); scaling rule (fixed, or min/max with a target); tier or mode where the Service has one (provisioned vs serverless, Standard vs Premium); cache size; partitions or shards for streams and databases; connection pool size.
- **Per Kind:** which fields apply to which Component Kind (a CDN has no instance count; a queue has no CPU).
- **Storage:** in the Node's `customData`, per ADR 0001 (the scene is the only source), as an optional `sizing` object. Unset means "use the Profile's default", so existing diagrams need no migration.
- **Outside Sim Mode:** is sizing shown on the canvas (a small badge such as "×3 · 4 vCPU"), editable in a Node's details, read by the Assistant in the compact format, and used by the Reliability & DR document and cost estimates?
- **Catalog:** do Cloud Catalog Services gain a list of instance types or sizes, and how big is that list?

## Resolution

Decided 2026-10-15, taking the recommended option throughout.

**Storage.** An optional `customData.sizing` object on Production Nodes (ADR 0001: the scene stays the only source). Every field is optional, and an unset field means "the Profile's default", so existing diagrams need no migration. Rough Draft Nodes never carry Sizing.

```json
"sizing": {
  "count": 3,                                      // instances, nodes or shards
  "size": "db.r6g.large",                          // a size id from the Profile (Service sizes, or Generic sizes)
  "scale": { "min": 2, "max": 10, "target": 0.6 }, // omitted = fixed
  "zones": 2,                                      // how many zones the instances spread over
  "pool": 20,                                      // connections each instance opens to each data store
  "concurrency": 500,                              // serverless functions
  "partitions": 4, "prefixes": 1,                  // quota-model stores
  "memory_mb": 512                                 // functions
}
```

**Which fields apply** comes from the Kind's Behaviour Profile, not from code: its capacity model decides it. `cpu` and `rate` take count, size, scale and zones; `quota` takes partitions or prefixes; `concurrency` takes concurrency and memory_mb; `managed` and `unbounded` take none. The Node details panel shows only the fields that apply.

**Sizes live in the Behaviour Profiles, not the Cloud Catalogs.** Each Service lists a short set of common sizes (3–6) with vCPU, RAM and price. The Generic target uses each Kind's Generic sizes. This keeps every simulation number in one place, and the catalogs stay about identity and compliance (ticket 017).

**Shown everywhere on the Production Diagram**, not only in Sim Mode, because sizing is part of the design:
- A small badge on the Node card (`×3 · r6g.large`, `auto 2–10`). Unset values show in grey as "default".
- Editable in the Node's details. Edits save a new version, like any other edit.
- In the compact format as one extra token group: `n2 "Orders DB" data.relational aws.rds-postgres ×1 db.r6g.large`. It's left out when unset, so token cost only grows where sizing exists.
- Read by the Reliability & DR Step (instance counts per zone feed composite availability) and by cost estimates.
- Proposals can set Sizing with a `set_sizing` op, which uses the same validation.
