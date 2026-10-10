---
id: "011"
title: "How does Conversion work?"
labels: [wayfinder:grilling]
status: closed
assignee: vinu (with claude)
blocked_by: ["002", "006", "007"]
parent: map
---

# How does Conversion work?

## Question

How does Conversion turn a frozen Rough Draft, the Design Brief and the Knowledge Base into the first Production Diagram? Covers: steps and prompting, the Knowledge Base-first then web-search order, how sources are cited, mapping generic shapes to catalog services, and the format and storage of the **Design Rationale** and **Gap Report** (met / unmet / outside what the diagram can show, with fix suggestions turned into Proposals).

_Note from "How is the Claude Code session configured?": the model can be switched mid-session (`set_model` control message), so Conversion could run on a stronger model in the same process. Decide here whether it should._

_Constraints from "What is the Knowledge Base record format?":_
- The Records that apply to a diagram are found by an `applies_to` lookup the runtime runs (Kinds, Data Classifications, cloud, Brief), not by search. Decide whether the runtime puts that list (IDs and summaries) into the prompt or the AI asks for it through a tool, given the ~6-tool budget.
- The AI judges met or unmet. Records marked `coverage: process` go straight to "outside what the diagram can show".
- A control's `satisfied_by` links to the practices that become fix suggestions, and the reverse `satisfies` link feeds the Design Rationale.
- Every Gap Report is stamped with the Knowledge Base version.

_From "Write the 5 pilot Knowledge Base records":_ `applies_to` matches by Kind, so practices over-apply (e.g. the S3 gateway endpoint practice matches every S3 Node on AWS, inside a VPC or not). Treat matched practices as **candidates** for the AI to judge, not findings. A control's `satisfied_by` list may be incomplete while the Knowledge Base is still being written.

## Resolution

Decided with vinu, 2026-10-10.

**Shape: two tasks, run once per Project**
- **Design**:
  - The AI types each Node (Kind, Service, Data Classification), designs the Production Diagram and writes the Design Rationale.
  - The user reviews the result in the **Conversion Preview**: the laid-out diagram, read-only, beside the Rationale. Review is all or nothing: **Accept**, **Reply** (new revision) or **Reject**, with no step-through and no accepting single parts.
  - Accept creates Production version 1 and **freezes the Rough Draft at that moment**. A rejected or abandoned Conversion leaves the draft editable.
- **Gap Check**:
  - Runs automatically once the Design is accepted, on that accepted version.
  - Can be run again on any later version with **Check again**. It never runs by itself after a Proposal (token cost).
  - A report on an older version shows a banner: "Checked against v3; the current version is v5".
- **Only one Conversion.** After it, every change goes through Proposals. Re-converting from the frozen draft is out of scope for v1.
- **Why two tasks:** the `applies_to` lookup needs Kinds, and Rough Draft Nodes have none. Splitting the work means the lookup runs in the runtime against a graph the user has accepted, so no Control can be skipped and the report always matches a real version.

**Before Conversion: a local pre-check (no AI, no tokens)**
- The runtime flags structural problems with fix-it prompts:
  - partly overlapping Nodes
  - unlabeled Nodes
  - loose arrows
  - Edges with no Node at either end
- Questions about meaning (e.g. is "DB" relational?) are not asked up front. The AI picks an answer and lists it under **Assumptions** in the Rationale, where the user can correct it with Reply.

**Model**
- A **Conversion model** setting, default **Opus**.
- The runtime uses `set_model` to switch to it for Design and Gap Check, then switches back to the per-install model.
- The cold cache this causes doesn't matter, because the first Conversion turn is cold anyway.

**The Design task message**
- **Static system prompt:** the ~40 Component Kinds.
- **Design message:** the Brief, the pre-checked Rough Draft in compact format, the target Cloud Catalog as a compact `id → kind` list (≈1.2k tokens; none for Generic), and the task instructions.
- **Practices:** found with `search_kb`, Knowledge Base first. WebSearch/WebFetch only when the Knowledge Base has nothing useful. No candidate list is put in the message, because there are no Kinds to filter by yet.

**Design Rationale**
- Structured, submitted with the Design as `rationale` on `submit_proposal`:
  `{summary, assumptions[], nodes: {alias: {why, practices:[ids], cites:[…]}}, tradeoffs:[{decision, chose, over, because}]}`
- The UI shows it as a document. Clicking a Node shows that Node's entry.
- **Kept current:** each accepted Proposal change updates the entries for the Nodes it touches (`why` and practices from `add_node`; removed Nodes lose their entry). Each version saves a snapshot.

**Citations**
- A citation is `{record: "<id>"}` or `{url, title}`.
- **The runtime validates every citation when it's submitted:**
  - a Record ID must exist in this Knowledge Base version
  - a URL's host must be on the trusted list
  - if either check fails, the submit is rejected with the reason, and the AI fixes it in the same turn
- **In the UI:** a Record citation opens a Record viewer with its own sources and licence credits. A URL is shown as a link marked "web".
- Web findings are **not** added to the Knowledge Base (R7 stays open).

**Gap Check task and Gap Report**
- **The Gap Check message:**
  - The runtime runs the `applies_to` lookup on the accepted graph and puts the Records that apply in the message (ID, title, summary, about 40 tokens each).
  - Records for security practices that matched are included as **candidates**.
  - The AI calls `get_record` only where it needs more detail.
- **Report header:** diagram version, Knowledge Base version, regimes, date.
- **Items:** one per Control that applies: `{record, status: met | unmet | outside, nodes:[aliases], finding (≤2 sentences), fix:[practice ids]}`.
  - Controls marked `coverage: process` are filled in by the runtime as `outside`, with no AI judgement.
  - A `met` Control marked `partial` shows "the diagram shows the component, not its configuration".
- **Security recommendations:** a second section for security practices that matched, aren't applied, and the AI judged relevant. This is how a Brief with compliance set to "none" still gets a useful report.
- **Storage:** JSON next to the version it checked, never edited afterwards. Paths are left to the "Project folder layout" item.

**Fixes**
- Fixes are drafted **on demand**:
  - **Fix** on one unmet item starts a short Assistant turn that drafts a single Proposal with `addresses` set to that item.
  - **Fix all** drafts one Proposal covering every unmet item selected.
- At most one Proposal is waiting at any time. Once the fix is accepted, the item shows "fix applied, check again".

**MCP tools (6):** `get_diagram`, `get_nodes`, `search_kb`, `get_record`, `submit_proposal` (gains an optional `rationale` and `kind: conversion`) and `submit_gap_report` (new). The Cloud Catalog and the list of Records that apply travel in task messages, so neither needs a tool.

**When things fail**
- **Progress:** the sidebar shows progress taken from tool calls.
- **Cancel:** sends `interrupt`.
- **No result:** a turn that ends without its submit counts as a failure, and the UI offers **Retry**.
- **Crash:** handled by the existing rule (resume once, then start fresh). A Gap Check simply starts over.
- **Large drafts:** no chunking in v1 (300 Nodes is about 15k tokens).

**What this hands to other tickets**
- **"How is the Production Diagram laid out and styled?"** must lay out a whole new diagram for the Conversion Preview, not only place new Nodes from a Proposal.
- **"What are the Component Kinds and Cloud Catalog contents?"** must produce a compact `id → kind` export of each Cloud Catalog for the Design message, and a Kinds list short enough for the system prompt.
