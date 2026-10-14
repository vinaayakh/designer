---
title: "Designer: the way to a Simulation spec"
labels: [wayfinder:map]
status: closed
---

# Designer: the way to a Simulation spec

## Destination

An **implementation-ready spec for Simulation**: a Sim Mode in the browser where the user runs traffic through a Production Diagram, watches it flow through each Node and Edge, tunes the infrastructure while it runs (more instances, bigger CPU and RAM, a different instance type, a cache in front of a database) and sees where it breaks. A monitoring dashboard shows traffic, errors and response times (p50, p95, p99). It plays like a simulation game (Factorio, Satisfactory, Anno) but stays a design tool: **rough numbers to find the right shape and the breaking points, not a precise performance prediction.** Targets live in [REQUIREMENTS.md](../../REQUIREMENTS.md) (R12); vocabulary in [CONTEXT.md](../../CONTEXT.md).

## Notes

- **Domain:** a Production Diagram already says *what* the system is made of (Component Kind, Service, Edges with sync/async/stream). Simulation adds *how much*: sizing on Nodes, a Behaviour Profile per Kind and Service, and traffic. Everything it needs is computed in the browser, with no tokens.
- **Settled when charting** (with vinu, 2026-10-14):
  - **Rough by design.** Results are shown as approximate (≈, ranges, bands). The point is "what breaks first, in what order, and roughly why", not "you need exactly 7 instances".
  - **A flow model, not a request-by-request simulation.** A fixed-tick fluid model (like Factorio belts) in a Web Worker moves traffic through the graph: queues fill, autoscaling reacts with a delay, timeouts cause retries, failures cascade. Each tick is one pass over Nodes and Edges.
  - **Tail latency comes from probe requests.** Each tick a sample of simulated requests is traced through the graph, drawing each Node's delay from its current latency distribution, so p95 and p99 reflect fan-out, retries and timeouts. Latencies are recorded in histograms and percentiles are read from merged histograms, never averaged.
  - **The user plays in a Scenario, a working copy of a Production version.** Edits in Sim Mode change the Scenario, not the diagram. **Apply** shows the Scenario's changes in the Proposal review overlay and saves one new version. The diagram only changes when the user accepts.
  - **Live, interactive play:** traffic slider and shapes (steady, daily cycle, spike), time controls (pause, 1×, 10×, 100×), events (kill a Node, lose a zone, slow a database), an inspector to change instance count, CPU/RAM size, instance type or Service, scaling rules and timeouts while it runs, and structural edits (add a cache, queue, read replica, load balancer).
  - **Monitoring dashboard:** traffic, error rate, p50/p95/p99 and cost, per entry point, per Node (rate, errors, duration) and per resource (utilisation, saturation, errors). Brief targets are drawn as SLO lines. Every chart carries event markers, including the user's own edits.
  - **Goals:** the Brief's latency, availability and budget targets are the pass or fail lines. No progression, unlocks or scoring beyond SLO and cost.
  - **Technology: no game engine.** TypeScript in a Web Worker for the simulation, a Canvas 2D overlay (WebGL only if the spike says so) on top of Excalidraw for flow dots and Node heat, and a small streaming chart library (uPlot) for the dashboard, all lazy-loaded with Sim Mode. Godot, Unity and similar were rejected: a web export ships a 30–40 MB WebAssembly runtime (against "must load fast"), can't share the Excalidraw canvas or the React UI, and adds a second toolchain to build. The simulation is arithmetic over a few hundred Nodes; it needs no physics, scene graph or game loop beyond a timer.
  - **The Assistant is optional here.** It can read a compact run summary and suggest changes, as a Proposal against the Scenario. The simulation never calls it.
- **Every session:** read `REQUIREMENTS.md`, `CONTEXT.md` and this map first. Use the Skill tool for `grilling` + `domain-modeling` on grilling tickets, and `prototype` where a ticket asks for one. Keep `REQUIREMENTS.md` (R12) and `CONTEXT.md` up to date as decisions land. vinu's standing preference: take the recommended option and proceed, recording the decision for review.
- **Standing preferences** (from the v1 map): performance first (fast load, quick on large diagrams) and token efficiency; nothing changes the diagram without the user's approval; Projects are plain files that diff well, runtime state in `.designer/`; content licensing per ADR 0002 (Behaviour Profile numbers are our own estimates with cited sources, never copied vendor tables).
- **Tracker conventions (local markdown):** same as [the v1 map](../map.md). Tickets are `wayfinder/sim/tickets/NNN-slug.md` with YAML frontmatter, `status: open|closed`, claim with `assignee:`, `blocked_by: [ids]`. The **frontier** is open, unassigned tickets whose `blocked_by` are all closed. Resolve with a `## Resolution` section, `status: closed` and a line under Decisions so far.

## Decisions so far

<!-- one line per closed ticket, then open the link for the detail the ticket holds -->

All 11 tickets were closed on 2026-10-15 in one pass. vinu asked for the map to be walked without feedback, taking best practice as the preferred option. **Every decision below is a recommendation awaiting vinu's review.** The spike, prototype and Profile check were not committed (`spikes/sim-001/`, `internal/catalog/profiles/`).

- [Can the sim run smoothly in the browser? (spike)](tickets/001-can-the-sim-run-smoothly-in-the-browser-spike.md): go on a Web Worker engine, a Canvas 2D overlay (no WebGL) and lazy uPlot (22.8 KB gzipped). 500 Nodes, 30 live charts, 3,195 dots and 100× speed hold 59–60 fps, with the overlay aligned to 0 px at DPR 1 and 2. Rules: sim state never re-renders the canvas; probes are budgeted per real second; charts show windowed percentiles; the overlay follows `snapshotPan`'s in-flight view; shed time follows caller timeouts.
- [What is the sim engine model?](tickets/002-what-is-the-sim-engine-model.md): a fixed 50 ms tick fluid model with six capacity models. Erlang C below saturation, backlog then shedding above; quota and concurrency limits reject at once. Retries scale with the measured timeout share, so retry storms emerge on their own. v1 limits: CPU, quotas, connections, memory. Autoscaling, cold starts, cache warm-up, zone loss, LB health checks, DB failover and circuit breakers are all in. Probes are stratified per Route. Seeded and replayable. One TypeScript implementation with golden tests on `node --test`.
- [How is a Node sized?](tickets/003-how-is-a-node-sized.md): optional `customData.sizing` (count, size, scale, zones, pool, concurrency, partitions…), with the fields that apply set by the Profile's capacity model and unset meaning the Profile default. Sizes live in the Profiles, not the Cloud Catalogs. Shown as a badge on every Production Node, in the compact format, and settable by Proposals (`set_sizing`).
- [What is a Behaviour Profile?](tickets/004-what-is-a-behaviour-profile.md): one YAML file per Kind with Service and size layers. Every number is `{low, typical, high}` with a `confidence` (vendor, measured, estimate), sources are facts-only, and prices are dated with `verify` flags. A Scenario picks typical, optimistic or pessimistic; spread is shown by comparing runs, not bands. Request weight is on the Edge. Project overrides in `sim/profiles.yaml`; a lint test with the copy check.
- [How does traffic enter and flow along Edges?](tickets/005-how-does-traffic-enter-and-flow-along-edges.md): named Routes (entry, share, latency target, per-Edge overrides) live in the Scenario. Edge behaviour is `customData.flow` (mult, weight, op, hit, async, parallel, timeout, retries, breaker), all inferred by default. Base traffic comes from a Note, then the Brief, then 100 rps flagged as an assumption. Shapes: steady, daily, spike, ramp, custom. Find breaking point is an action.
- [What is a Scenario and how is it stored?](tickets/006-what-is-a-scenario-and-how-is-it-stored.md): `sim/scenarios/<slug>.yaml` holds the base version, seed, Profile mode, traffic, Routes, events and a change list in the Proposal op vocabulary. The working scene is built in memory. Rebase on demand lists dropped changes. "Normal day" is created on first use. The latest run is kept in `.designer/sim/`.
- [How are metrics collected and shown on the dashboard?](tickets/007-how-are-metrics-collected-and-shown-on-the-dashboard.md): RED and USE metrics plus backlog age, error-budget burn and cost; ±1% log histograms with exact per-window percentiles; retention of 1 s × 5 min, 10 s × 1 h and 1 min × 24 h (about 25 MB at 500 Nodes). A dock under the canvas with tiles, Route charts, a service table and Node detail. SLO hysteresis; event markers for every edit; compare against a baseline run in v1; time scrubbing later.
- [What does Sim Mode look like?](tickets/008-what-does-sim-mode-look-like.md): a Simulate button opens Sim Mode in place, with a read-only canvas and our own hit-testing. The overlay draws outline heat, a utilisation bar, badges and flow dots, switching to Edge thickness below 30% zoom. Control bar, a Sim tab in the inspector, and a palette (cache, queue, read replica, LB, CDN, breaker) with insertion rules. A breaking-point summary; Exit keeps the Scenario. The spike page is the prototype.
- [How do Scenario changes get back into the diagram?](tickets/009-how-do-scenario-changes-get-back-into-the-diagram.md): Apply turns the diagram changes (structure, Sizing, Edge behaviour) into a user-authored Proposal in the unchanged review overlay. It needs a rebased Scenario, and after Apply the Scenario rebases. The runtime offers `open` `DEC` entries written from the run's numbers.
- [What does the Assistant see of a run?](tickets/010-what-does-the-assistant-see-of-a-run.md): a run summary of about 300 tokens built in the browser and stored with the run, sent only on request. `get_sim_summary` reads it; there is no run-the-sim tool. `submit_proposal` can target a Scenario. Bottleneck patterns map to Practice IDs for grounded advice.
- [Write the starter Behaviour Profiles](tickets/011-write-the-starter-behaviour-profiles.md): 10 Kinds with sourced, ranged numbers. On three example systems the saturation orders are believable: CRUD api at ≈670 rps; catalog api at ≈1.5k CDN rps; pipeline S3 → DynamoDB → API Gateway. It found five fixes: explicit capacity models, usage-based prices, quotas reject rather than queue, backlog age as a goal, partitions as visible Sizing. A second batch brings it to 31 Kinds (every Kind on an AWS request path, plus third-party APIs), adds the `hop` model and one quota shape, and passes a lint. Six example systems behave believably; latency targets break before saturation, and cost readouts must flag Services without prices.

**AWS Components and the Component Library** (added 2026-10-15). vinu asked for the sim to use AWS Components with AWS's icons, through a searchable library grouped by load balancer, compute, ECS, EKS and so on, configured so Azure can follow. The same mode applies: recommended options, for review.

- [What is a Component, and how does a cloud plug in?](tickets/012-what-is-a-component-and-how-does-a-cloud-plug-in.md): a Component is a ready-to-place preset (Service or Kind, icon, label, keywords, group, placement, sim coverage, defaults), and several can share a Service. Five layers; only the library, catalog and icon manifest know a cloud. Adding a cloud means four data files and no code. Library groups carry a neutral `role`. A draft Azure library on the same schema renders from the same code.
- [Which AWS components are in the library, and how are they grouped?](tickets/013-which-aws-components-are-in-the-library.md): 83 Components in 13 groups plus Quick inserts: Traffic sources, Load balancing, Edge and APIs, Compute, ECS, EKS, Databases, Caching, Messaging and streaming, Storage, Network, Security and identity, Monitoring. Other Services sit under More. Each Component is badged Simulated, Pass-through or Diagram only. Adds `aws.eks-workload` and `azure.aks-workload` (overrides v1 ticket 017's Service list). 10 Kinds marked Simulated still need Profiles.
- [How do ECS and EKS simulate?](tickets/014-how-do-ecs-and-eks-simulate.md): a cluster is a Group (new group kind `cluster`, overriding v1 ticket 017's list) with an optional EC2 capacity pool. Tasks and pods must fit the pool's vCPU; unplaced ones wait for hosts (about 150 s boot). Request-count scaling is the default behind a load balancer. Measured on a 600 → 3,000 rps spike: Fargate 141 s over SLO, tight EC2 pool 252 s, EC2 with headroom 100 s; CPU scaling was slower in every case.
- [How are AWS icons fetched, cached and drawn?](tickets/015-how-are-aws-icons-fetched-cached-and-drawn.md): the pinned AWS Architecture Icons package `07312026` (13.99 MB, sha256 checked), keyed by file name. The manifest lists the 85 files used. The Go runtime downloads after consent and extracts into the user cache (468 KB). Icons are embedded in the scene, locked, and always shown with the full name; cards wrap the name instead of truncating it (refines v1 ticket 016). An icon-sync script and CI key check follow AWS's quarterly releases.
- [What does the Component Library panel look like?](tickets/016-what-does-the-component-library-panel-look-like.md): one panel on the left, in Production editing and Sim Mode (not the Rough Draft). Search, Quick-insert chips and collapsible groups; rows with icon, name, sim badge and HIPAA warning. Weighted word search with results kept grouped (≈1 ms per keystroke). Drag or click to place, with insertion rules on Edges and Nodes. It replaces the Inspector's Add dropdowns and Sim Mode's palette. Prototype: `spikes/sim-002-library/`.

## Built so far

**First slice (2026-10-15),** built at vinu's request so a running diagram can go to the sim and back ([docs/features/simulation.md](../../docs/features/simulation.md)). It covers:
- the **Simulate** button and Sim Mode, read-only over the latest version;
- the diagram import (`web/src/sim/resolve.ts`);
- the engine in a Web Worker;
- baseline and test runs with the breaking point;
- changes to sizing, autoscaling, calls per request, and a cache in front of a store;
- **Apply to diagram** as a user-authored Proposal (Accept or Skip, no Reply);
- **Rebase** after it's accepted.

It was tested end to end on a copy of a real Project. Where it differs from the tickets, on purpose for a first slice:
- **Sizing is stored as readable Node attributes** (`instances`, `autoscaling`, `instance`), not ticket 003's `customData.sizing` object. Diagrams the Assistant writes already carry sizing as free text (`tasks: 2-4`), so the import reads both, Apply needs no new Proposal operation, and the Assistant can read the result. Ticket 003's typed object can still replace it later, with the attributes as the migration path.
- **The Scenario stores maps,** not ticket 006's op list: sizing and flow overrides keyed by element ID, plus added caches. Apply translates them into Proposal operations. There's one Scenario per Project (`baseline`), in the typical Profile mode only.
- **Not built yet:**
  - the Component Library panel and AWS icons (tickets 013–016);
  - events and traffic shapes;
  - charts and the compare-runs overlay (ticket 007);
  - structural edits other than a cache;
  - capacity pools in the UI (ticket 014);
  - the Assistant's run summary (ticket 010);
  - the `snapshotPan` hook. Sim Mode uses its own canvas instance, without snapshot panning.

## Tickets

| # | Ticket | Kind | Status |
|---|---|---|---|
| 001 | [Can the sim run smoothly in the browser? (spike)](tickets/001-can-the-sim-run-smoothly-in-the-browser-spike.md) | task | closed |
| 002 | [What is the sim engine model?](tickets/002-what-is-the-sim-engine-model.md) | grilling | closed |
| 003 | [How is a Node sized?](tickets/003-how-is-a-node-sized.md) | grilling | closed |
| 004 | [What is a Behaviour Profile?](tickets/004-what-is-a-behaviour-profile.md) | grilling + research | closed |
| 005 | [How does traffic enter and flow along Edges?](tickets/005-how-does-traffic-enter-and-flow-along-edges.md) | grilling | closed |
| 006 | [What is a Scenario and how is it stored?](tickets/006-what-is-a-scenario-and-how-is-it-stored.md) | grilling | closed |
| 007 | [How are metrics collected and shown on the dashboard?](tickets/007-how-are-metrics-collected-and-shown-on-the-dashboard.md) | grilling | closed |
| 008 | [What does Sim Mode look like?](tickets/008-what-does-sim-mode-look-like.md) | grilling + prototype | closed |
| 009 | [How do Scenario changes get back into the diagram?](tickets/009-how-do-scenario-changes-get-back-into-the-diagram.md) | grilling | closed |
| 010 | [What does the Assistant see of a run?](tickets/010-what-does-the-assistant-see-of-a-run.md) | grilling | closed |
| 011 | [Write the starter Behaviour Profiles](tickets/011-write-the-starter-behaviour-profiles.md) | task | closed |
| 012 | [What is a Component, and how does a cloud plug in?](tickets/012-what-is-a-component-and-how-does-a-cloud-plug-in.md) | grilling | closed |
| 013 | [Which AWS components are in the library, and how are they grouped?](tickets/013-which-aws-components-are-in-the-library.md) | grilling | closed |
| 014 | [How do ECS and EKS simulate?](tickets/014-how-do-ecs-and-eks-simulate.md) | grilling + research | closed |
| 015 | [How are AWS icons fetched, cached and drawn?](tickets/015-how-are-aws-icons-fetched-cached-and-drawn.md) | grilling + research | closed |
| 016 | [What does the Component Library panel look like?](tickets/016-what-does-the-component-library-panel-look-like.md) | grilling + prototype | closed |

## Before building

- **Verify prices:** check every `verify: true` price against the provider's pricing page, and source Azure prices.
- **More Profiles:** document database and the remaining Kinds, with the same check on example systems.
- **Snapshot pan hook:** `web/src/snapshotPan.ts` needs a subscribe hook for the in-flight view (ticket 001).
- **Prices:** 21 prices are marked `verify`, and 53 Services have none, including DocumentDB, OpenSearch, Neptune, MSK, Kinesis, EFS and all of Azure. The cost readout must say when its total leaves Services out (ticket 011).
- **Kinds still without a Profile** (17) are all diagram-only or traffic sources today: DNS, key management, secrets, DDoS, threat detection, monitoring (4), backup, warehouse, batch, notifications, and the external user, client and system sources. `compute.ai-model` (Bedrock, SageMaker) is the one that could sit on a request path; it needs a Profile before it joins the library.
- **Catalog changes:** add `aws.eks-workload` and `azure.aks-workload`, the `cluster` group kind, and the catalog's `icon_set` and Service `icon` keys from `internal/catalog/icons/aws.yaml`.
- **Azure:** Profile overrides and prices, an icon manifest for the Azure Architecture Icons package, and VM and node-pool sizes. The library draft is `internal/catalog/library/azure.yaml`.
- **New Practice Records** the Assistant mapping needs (ticket 010): read replicas, queue-based load levelling, circuit breaker, backoff with jitter, partition key design. Only cache-aside exists today.

## Not yet specified

- **Calibration.** Whether, later, users can feed in real numbers (a load test result, production metrics) to tune a Profile for their Project, and how that is kept apart from the built-in Profiles.
- **Request-level zoom.** A request-by-request simulation of a small part of the graph, for cases where probe sampling isn't enough. Only if the engine ticket finds a real need.
- **Simulation in documents.** Whether a run's results (bottleneck order, headroom, dashboard snapshot) can be cited as evidence in the Reliability & DR document, and how a Scenario run becomes a Register entry (`DEC` or `RISK`).
- **Data growth over time.** Storage filling up, index size versus memory, and other slow effects measured in months rather than seconds.
- **Sharing a run.** Exporting a replay (Scenario + seed + edit log) so someone else can watch it.

## Out of scope

- Accurate performance prediction or capacity planning sign-off: Simulation is a sketching tool.
- Load testing or connecting to real infrastructure.
- Simulating application code, query plans or specific database engines' internals.
- Game features beyond SLO and cost goals (levels, unlocks, scores, achievements).
- A game engine or any runtime beyond the browser (see Notes).
