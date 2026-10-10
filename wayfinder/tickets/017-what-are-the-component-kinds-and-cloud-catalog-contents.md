---
id: "017"
title: "What are the Component Kinds and Cloud Catalog contents?"
labels: [wayfinder:grilling]
status: open
assignee:
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
