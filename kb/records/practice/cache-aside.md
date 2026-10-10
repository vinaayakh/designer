---
id: practice.cache-aside
type: practice
domain: design
title: Cache-aside (lazy loading)
summary: The application reads from a cache first, loads from the data store on a miss, and invalidates the cached item when it writes.
aliases: [lazy loading, look-aside cache, read-through cache, cache miss, cache invalidation, TTL, expiry, eviction, Redis, Memcached, read-heavy, hot data]
coverage: checkable
applies_to:
  kinds: ["data.relational", "data.document", "data.keyvalue", "data.cache"]
  brief: { read_write_bias: [read-heavy] }
sources:
  - title: "Cache-Aside pattern, Azure Architecture Center"
    url: https://learn.microsoft.com/en-us/azure/architecture/patterns/cache-aside
    as_of: 2025-09-11
    licence_tier: attribution
    attribution: "Adapted from \"Cache-Aside pattern\" by Microsoft, Azure Architecture Center (MicrosoftDocs/architecture-center @ 0ab15d0), licensed CC BY 4.0. Changes: condensed and rewritten for diagram review."
revised: 2026-10-10
---

## Practice

Put a cache beside a data store that serves repeated reads. To read, the application checks the cache. On a miss it reads the store, writes the item to the cache, and returns it. To write, it updates the store and then removes the cached item, so the next read reloads it.

Use it when the cache has no built-in read-through or write-through, or when demand for data is hard to predict. Avoid it when:
- the data is sensitive and the cache is shared (read it from the primary store);
- the data is static and fits in memory (prime the cache at startup instead);
- most reads would miss anyway;
- it would hold web-farm session state, which ties clients to servers.

## How to check on a diagram

- A `data.cache` Node sits between the compute that reads and the store it reads from: compute has edges to both the cache and the store.
- The store remains the source of truth; nothing writes only to the cache.
- For read-heavy Briefs, a hot read path with no cache is worth a remark.

## Suggested fixes

- Add a cache Node beside the store, with an edge from the reading service.
- Note the expiry and invalidation strategy in the Design Rationale.

## Trade-offs

- The cache and store can disagree: after a write, readers may miss or briefly see stale data. Write-through avoids this but costs a write to both.
- Expiry too short wastes store reads; too long serves stale data.
- Adds another component to run and monitor. A local in-process cache gives each instance its own view of the data.
