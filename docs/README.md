# Designer documentation

Designer is a local, AI-assisted architecture tool: sketch a system, answer a short brief, and get a cloud-specific Production Diagram, a compliance Gap Report and the engineering documents that go with it, with an audit trail behind every claim. AI work runs in your own Claude Code.

## [Feature guides](features/README.md)

- [Getting started](features/getting-started.md): install, open a Project, the screen layout, cost
- [Rough Draft, Brief and Conversion](features/conversion.md)
- [Proposals and versions](features/proposals.md)
- [Gap Check](features/gap-check.md)
- [Documents and the Register](features/documents.md)
- [Export](features/export.md): images, arc42, the document pack
- [Limitations in this version](features/README.md#limitations-in-this-version)

## Internals

- [Architecture](internals/architecture.md): packages, how Claude Code is run and sandboxed, tests
- [The Register and the Document Pipeline](internals/register-and-pipeline.md)
- [Logging and performance](internals/logging-and-performance.md): logs, metrics, flags, the pan benchmark

## Decisions

- [Architecture decision records](adr/)
- [Planning maps](../wayfinder/map.md): v1 and the [Document Pipeline](../wayfinder/doc-pipeline/map.md), with every decision, the alternatives and the research behind them
- [Requirements](../REQUIREMENTS.md) and [vocabulary](../CONTEXT.md)

## Project

- [Contributing](../CONTRIBUTING.md) · [Security](../SECURITY.md) · [Publishing a release](../PUBLISHING.md) · [Changelog](../CHANGELOG.md) · [Guide for AI agents](../AGENT.md)
