---
id: "012"
title: "What is a Component, and how does a cloud plug in?"
labels: [wayfinder:grilling]
status: closed
assignee: claude (recommended options, for vinu's review)
blocked_by: ["004"]
parent: map
---

# What is a Component, and how does a cloud plug in?

## Question

vinu (2026-10-15): the sim should use **components from AWS**, with AWS's icons, from a **component library**. It must be configured so that **Azure can be supported later**. What is a Component, and where does each cloud's knowledge live, so that a second cloud is data rather than new code?

## Resolution

Decided 2026-10-15, taking the recommended option throughout.

**A Component is a ready-to-place preset from a cloud's Component Library:**
- a catalog Service (identity, Component Kind, HIPAA flag), or a Kind for traffic sources and Groups;
- an official icon key;
- a short label and search keywords;
- a library group;
- how it's placed (Node, Group or traffic source);
- its simulation coverage (`full`, `hop` or `off-path`);
- the Sizing, pool and Edge-behaviour defaults it arrives with.

Several Components can share one Service. "EC2 instance" and "EC2 Auto Scaling group" are both `aws.ec2` with different default Sizing. The diagram stores `customData.component` alongside `kind` and `service`, so the library can show the right icon again, but nothing depends on it.

**Layers.** Only the top layer knows a cloud:

| Layer | Knows | Files |
|---|---|---|
| Sim engine | capacity models, Sizing, Edge behaviour (no Kind or cloud names) | `web/src/sim/` |
| Behaviour Profiles | Kinds, plus per-Service overrides keyed by Service ID | `internal/catalog/profiles/<kind>.yaml` |
| Cloud Catalog | Services, their Kind, HIPAA flag, aliases | `internal/catalog/data/<cloud>.yaml` |
| Component Library | groups, Components, icons, quick inserts | `internal/catalog/library/<cloud>.yaml` |
| Icon manifest | the pinned vendor icon package and the files used | `internal/catalog/icons/<cloud>.yaml` |

A resolver turns (Component or Service, Sizing, Profile mode) into the engine's numbers: Kind Profile, then Service override, then size, then Project override (ticket 004). It's the only place these files meet.

**Adding a cloud** means adding four data files: catalog, library, Profile overrides for its Services, and an icon manifest. No engine, panel or renderer code changes. Library groups carry a neutral `role` (`load-balancing`, `compute`, `containers`, `kubernetes`, `databases`, `caching`…), so features that cross clouds can line groups up by role, like the Assistant suggesting the Azure equivalent of an ECS service (Container Apps or AKS).

**Proof.** [library/azure.yaml](../../../internal/catalog/library/azure.yaml) is a draft Azure library on the same schema:
- it has Azure's own groups (AKS and Container Apps where AWS has ECS and EKS);
- it uses only real Azure catalog IDs, apart from the pending `azure.aks-workload` (ticket 014);
- its icons are null, so it uses generic icons.

The prototype (`spikes/sim-002-library/`) renders it from the same code as AWS.

**Azure still needs** before it ships: Profile overrides and prices for its Services, an icon manifest for the Azure Architecture Icons package, and Azure Sizing names (VM sizes, node pool SKUs). These are listed on the map under "Before building".

**Generic target.** With no cloud, the library is built from `kinds.yaml`: one Component per Kind, grouped by category, with Tabler icons. The sim works the same from the Kind Profiles alone.
