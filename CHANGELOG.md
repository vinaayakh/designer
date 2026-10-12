# Changelog

All notable changes to Designer are recorded here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow [Semantic Versioning](https://semver.org); while Designer is 0.x, a minor version may change Project file formats.

## [Unreleased]

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
