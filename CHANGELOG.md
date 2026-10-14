# Changelog

All notable changes to Designer are recorded here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow [Semantic Versioning](https://semver.org); while Designer is 0.x, a minor version may change Project file formats.

## [Unreleased]

### Added

- **Sim Mode** (Simulate in the top bar): runs traffic through the latest Production version in the browser and shows it flowing on the diagram. It reads sizing from the diagram's attributes and the Brief, records a **baseline** (latency, errors, cost and the breaking point), lets you change instances, size, autoscaling and calls per request or add a cache, and compares a **test** run against the baseline. The Scenario is saved in `sim/scenarios/baseline.yaml`.
- **Apply to diagram** turns a Scenario's changes into a Proposal marked as yours, reviewed with Accept or Skip; sizing is written as `instances`, `autoscaling` and `instance` attributes.
- Behaviour Profiles for 31 Component Kinds (`internal/catalog/profiles`), embedded and checked by a lint test.
- **Detached parts** of a diagram (a Node with no Edges, or Nodes linked only to each other) are found by `scene.Graph.Detached()` and reported to the Assistant as `! detached` issues, including when they appear mid-session. The Assistant leaves account-wide services (audit logging, threat detection, key management) in place, proposes removing clear leftovers for your review, and asks when unsure. Detached parts don't block Conversion.
- **Watchers:** a new Edge style, `observe` ("watches", drawn dotted violet), which can start from a Group. Account-wide services such as CloudTrail are drawn the way AWS reference architectures draw them: inside the AWS account, with one observe Edge from the account instead of an arrow to every service. The Assistant follows this, the simulation ignores observe Edges, and a watcher observing a Group no longer counts as detached.
- API: `POST /api/proposal/user` submits a Proposal you wrote (outside the Assistant) for the usual review.
- API: `GET /api/sim/profiles`, `GET`/`PUT /api/sim/scenarios/{slug}`, `POST /api/sim/apply`.

### Fixed

- After accepting a Proposal, the canvas could show the previous version until you reloaded: the server's "Proposal closed" event started a load of the old version that finished after the new one was drawn.
- A Proposal that moves a Group into another (for example a new AWS account around the region) is now laid out fully, like Tidy up, both in review and when accepted; before, the moved Groups kept their old boxes.

## [0.1.0] - 2026-10-12

The first public release.

### Added

- **Rough Draft** canvas on Excalidraw: shapes, arrows and enclosing shapes become Nodes, Edges and Groups.
- **Design Brief** wizard: target cloud (AWS, Azure, generic), scale, latency, availability, RPO/RTO, residency, compliance (SOC 2, HIPAA), budget and team maturity, with `Unknown` recorded as an assumption.
- **Conversion** into a typed, automatically laid-out Production Diagram with a Design Rationale, reviewed as a whole in a Conversion Preview.
- **Proposals** for every AI change, reviewed change by change on the canvas; versions for every accepted Proposal and hand edit.
- **Gap Check** against SOC 2 and HIPAA controls and security practices, with **Fix** to draft a Proposal.
- **Document Pipeline**: Requirements spec, Design doc + ADRs, Threat model (STRIDE) and Reliability & DR, each with its own questions, written by the Assistant, versioned, and marked stale when its inputs change.
- **Register**: an append-only, hash-chained audit trail of requirements, constraints, assumptions, non-goals, decisions, risks and open questions. Entries can be proposed from chat and are only recorded once accepted.
- **Export**: PNG and SVG; the arc42 document as Word or PDF; a document pack (zip) with every document, MADR 4.0 ADRs, the Register and an index.
- Logs, metrics and resource sampling (`--verbose`, `--slow-ms`, `--sample`, `GET /api/metrics`), and smooth panning and zooming on large diagrams.
- Release archives for Linux, macOS and Windows (amd64, arm64) with checksums and build provenance, and installers for each.

[Unreleased]: https://github.com/vinaayakh/designer/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/vinaayakh/designer/releases/tag/v0.1.0
