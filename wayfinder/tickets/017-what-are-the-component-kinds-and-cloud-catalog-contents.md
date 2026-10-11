---
id: "017"
title: "What are the Component Kinds and Cloud Catalog contents?"
labels: [wayfinder:grilling]
status: closed
assignee: vinu (with claude)
blocked_by: []
parent: map
---

# What are the Component Kinds and Cloud Catalog contents?

## Question

Using the two type layers from "What is the diagram data model?":
1. What is the v1 **Component Kind** vocabulary (about 40 kinds: names, categories, which ones store data and so can carry a Data Classification) and the **group kinds** (region / vpc / vnet / subnet / az / account, plus anything else)?
2. What fields does a **Cloud Catalog** entry have? Candidates: service ID, display name, Kind, provider category, icon reference, HIPAA-eligible plus an as-of date, a link to the docs. Which AWS and Azure Services are in v1, and how many?
3. How is the catalog kept in step with the icon releases (the scrape-and-diff script from "How do we draw the Production Diagram?")? Icon *distribution* is decided in "What are the licensing rules for the Knowledge Base and icons?"

Constraints from "What are the licensing rules for the Knowledge Base and icons?":
- AWS/Azure icons are **downloaded on first use** after the user agrees to the vendor's terms, never bundled. A catalog entry's icon reference points into that downloaded set (stable key, not a bundled path), and the download manifest (URL, version, checksum) is kept by the icon-sync script.
- Every Component Kind needs a **generic fallback icon** (used offline, or when the user declines the terms), shown with the full service name.
- The full official service name must be stored on each catalog entry, since it's shown beside official icons.

_Constraints from "What is the Knowledge Base record format?":_ Records match Kinds by glob (`applies_to.kinds: ["data.*"]`), so Kind IDs need dotted namespaces where the prefix means something (`data.relational`, `data.object`…). Records also match on Data Classification values, so that list must be fixed here as well (PHI, PII, …).

_From "Write the 5 pilot Knowledge Base records":_ the pilot Records use the provisional Kinds `data.relational`, `data.document`, `data.keyvalue`, `data.cache` and `data.object`, and the Data Classes `PHI`, `PII` and `confidential`. Rename them in `kb/records/` if this ticket picks different IDs. A Data Classification must be allowed on **any Node that holds data** (queues, caches, search indexes), not only on `data.*` Kinds, because the PHI Records now match on Data Classification alone.

_From "How does Conversion work?":_ the Kind list (IDs plus a few words each) goes in the **static system prompt**, so keep it short. Each Cloud Catalog needs a **compact `id → kind` export** (about 8 tokens per Service) that goes in the Design task message. Aim for about 1.2k tokens per cloud.

_From "How is the Production Diagram laid out and styled?":_
- **Generic icons:** each Component Kind and group kind needs a **Tabler** icon (MIT); only the mapped SVGs ship.
- **Category:** each Kind also needs a category for the card's border tint: compute / data / network / security / messaging / external.

## Resolution

Decided with vinu, 2026-10-10.

**Kind IDs**
- `<category>.<name>`, kebab-case. The prefix **is** the category, so one fact drives both Record globs (`data.*`) and the card's border tint.
- **Seven categories:** the six from the layout ticket plus **`observability`**. Clients and third-party SaaS are `external.*`; analytics is `data.warehouse` / `compute.batch`.
- The pilot's `data.keyvalue` is renamed `data.key-value` in `kb/records/`.

**The v1 Kind list (48)**

| Category | Kinds |
|---|---|
| compute (7) | `vm`, `container`, `kubernetes`, `function`, `web-app`, `batch`, `ai-model` |
| data (11) | `relational`, `document`, `key-value`, `graph`, `cache`, `object`, `file`, `search`, `warehouse`, `time-series`, `backup` |
| network (9) | `dns`, `cdn`, `load-balancer`, `api-gateway`, `internet-gateway`, `nat`, `private-endpoint`, `transit`, `hybrid-link` |
| security (7) | `identity`, `key-management`, `secrets`, `waf`, `firewall`, `ddos`, `threat-detection` |
| messaging (6) | `queue`, `topic`, `stream`, `event-bus`, `workflow`, `notification` |
| observability (4) | `logs`, `metrics`, `tracing`, `audit` |
| external (4) | `user`, `client`, `service`, `system` |

`kinds.yaml` entry: `id`, a short `gloss` (goes in the static system prompt), Tabler `icon`, `holds_data`.

**Data Classification**
- Multi-select tags: **`PHI`, `PII`, `confidential`, `public`**. PHI doesn't imply PII. No tag means *unclassified*, and the AI states its assumption. No `PCI` (no PCI regime in v1) and no `secrets` (that's the `security.secrets` Kind).
- The card badge shows the most sensitive tag: PHI > PII > confidential.
- Allowed only on Kinds with `holds_data: true`: every `data.*`; `messaging.queue/topic/stream/event-bus`; `observability.logs/audit`; `external.service/system` (where it means "receives data of this class", which is what raises the BAA question).

**Group kinds (provider-neutral)**
- `region`, `zone`, `network` (shown as VPC/VNet from the catalog), `subnet` with tier `public|private|isolated`, `account` (AWS account / Azure subscription), `boundary` (a trust zone: on-prem, third party, DMZ).
- No resource groups and no cluster groups in v1: a cluster is a `compute.kubernetes` Node.
- The compact format becomes `g2 network "Prod VPC"`, `g3 subnet:private "App"`.

**Cloud Catalog**
- Files: `catalog/kinds.yaml`, `catalog/aws.yaml`, `catalog/azure.yaml`, built into the binary.
- Catalog header: `provider`, group-kind display names, `hipaa_as_of`, pinned `icon_set` (version, URL, sha256).
- Entry:
  ```yaml
  - id: aws.rds-postgres
    name: Amazon RDS for PostgreSQL   # full official name, shown beside the icon
    kind: data.relational
    icon: Arch_Amazon-RDS              # stable key into the downloaded set; null → generic
    hipaa: true
    docs: https://docs.aws.amazon.com/...
    aliases: [rds, postgres, postgresql]
  ```
  There's no provider-category field, because the Kind gives the category.
- **Multi-engine products are one entry per engine or API** (`azure.cosmos-nosql → data.document`, `azure.cosmos-postgres → data.relational`), sharing an icon. "Exactly one Kind per Service" holds.
- **Off-catalog services:** a Node may have a Kind plus a free-text service name, with the generic icon and a *not in catalog* mark. The runtime validates the Kind only. Third-party SaaS always works this way. Compact form: `n7 "Payments" external.service ~Stripe`.
- **Size:** about 80–120 Services per cloud. Rules: every Kind is covered, the reference architectures for web apps, data and serverless are covered, every HIPAA-eligible service that maps to a Kind is in, niche domains are out. CI checks coverage and that the `id → kind` export stays ≤ 1.2k tokens. The entries themselves are written during implementation.

**Icons and sync**
- Generic: each Kind and group kind names one Tabler icon (about 53 ship). Icons may be shared. CI fails on a missing or unknown icon. The specific picks are made during implementation.
- The icon-sync script is dev-time only. It downloads a vendor release and writes a diff report (new, removed, renamed, dangling catalog keys). People update the catalog and bump `icon_set`. CI fails on a dangling key. At runtime a missing key falls back to the generic icon.
- Refreshing the HIPAA-eligible list (`hipaa_as_of`) belongs with the "Keeping the Knowledge Base current" fog.
