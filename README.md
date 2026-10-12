# Designer

Designer is a local, AI-assisted architecture tool. Sketch a system as a hand-drawn **Rough Draft**, answer a short **Design Brief**, and Designer turns it into a cloud-specific **Production Diagram** with a Design Rationale, a SOC 2 / HIPAA **Gap Report**, and the documents a senior engineer is expected to write: a requirements spec, a design doc with ADRs, a STRIDE threat model, a reliability and disaster-recovery plan, and an arc42 architecture document. Every requirement, assumption, decision and risk lands in an append-only, hash-chained **Register**, so you can always show where a document's claims came from.

AI work runs in **your own [Claude Code](https://claude.com/claude-code)**: Designer never sees or stores an API key, and Claude Code only gets Designer's own tools plus web search on a short list of trusted sources.

- Single binary, no runtime dependencies besides Claude Code. The UI opens in your browser.
- Projects are plain files (YAML, JSON, `.excalidraw`, Markdown) that diff well in git.
- Nothing changes your diagram until you approve it: every AI change is a Proposal you review change by change.
- Built on [Excalidraw](https://excalidraw.com) for the hand-drawn canvas and [ELK](https://eclipse.dev/elk/) for layout.

> **Status:** early (0.x). The workflow is complete end to end; the Knowledge Base has a handful of pilot Records, and official AWS/Azure icons are not bundled yet (generic icons are used). Expect file formats to change before 1.0.

## Features

- **Rough Draft:** a fast, freeform Excalidraw canvas. Shapes become Nodes, arrows become Edges, and shapes drawn around others become Groups.
- **Design Brief:** a short wizard for the target cloud (AWS, Azure or generic), scale, latency, availability, RPO/RTO, data residency and compliance. `Unknown` is always an answer; it becomes a recorded assumption.
- **Conversion:** the Assistant turns the sketch into a typed, laid-out Production Diagram, with a rationale for every component, citing the Knowledge Base. You accept, reply to or reject it as a whole.
- **Gap Check:** checks the design against SOC 2 and HIPAA controls and security practices, with findings tied to diagram elements and a one-click **Fix** that drafts a Proposal.
- **Proposals:** ask for changes in chat; review each change on the canvas before it's applied. Every accepted Proposal or edit is a new version.
- **Documents:** a step-by-step pipeline that writes a Requirements spec, a Design doc + ADRs, a Threat model (STRIDE) and a Reliability & DR plan, each building on the last. Each asks its questions first, cites the Register, and goes stale when anything it relied on changes.
- **Register:** the audit trail. Requirements, constraints, assumptions, non-goals, decisions, risks and open questions, each with its source and full history. Only you confirm or reject what the Assistant adds.
- **Export:** PNG and SVG; the arc42 document as Word or PDF; a **document pack** (zip) with every document, an ADR per decision (MADR 4.0), the Register and an index for reviewers or auditors.

See the [feature guides](docs/features/README.md) for walkthroughs.

## Installation

You need [Claude Code](https://claude.com/claude-code), installed and logged in.

macOS and Linux:

```sh
curl -fsSL https://raw.githubusercontent.com/vinaayakh/designer/main/scripts/install.sh | sh
```

Windows (PowerShell):

```powershell
irm https://raw.githubusercontent.com/vinaayakh/designer/main/scripts/install.ps1 | iex
```

Both installers verify the download against the release's `checksums.txt`. You can also download an archive from [Releases](https://github.com/vinaayakh/designer/releases) and check its build provenance with `gh attestation verify <file> --repo vinaayakh/designer`.

Build from source (Go 1.26+, Node 22+):

```sh
git clone https://github.com/vinaayakh/designer.git
cd designer
make build            # or scripts/build.sh on Windows (Git Bash)
bin/designer --version
```

## Usage

```sh
designer ~/my-system             # open (or create) a Project folder and the browser
designer --port 5000 ~/my-system # preferred port (falls back to a free one)
designer --no-open ~/my-system   # don't open the browser
designer --verbose ~/my-system   # debug logs: requests, tool calls, timings
designer --claude /path/to/claude ~/my-system  # a Claude Code binary not on PATH
designer --version
```

The server listens on 127.0.0.1 only. Logs go to `<project>/.designer/logs/designer.log`; see [Logging and performance](docs/internals/logging-and-performance.md) for every flag.

A Project folder is plain files you can commit:

```
project.yaml                    name, created, when the Rough Draft was frozen
brief.yaml                      the Design Brief
rough-draft.excalidraw
production/vNNN.excalidraw      one per Production version, plus vNNN.json (rationale)
gap-reports/vNNN-<time>.json    never edited after they are written
register/log.jsonl              the Register: append-only, hash-chained
documents/<step>/vNNN.json      each document version, with a Markdown copy (vNNN.md)
.designer/                      runtime state and logs (git-ignored)
```

## Documentation

- [Docs hub](docs/README.md): feature guides, internals and decision records
- [Requirements](REQUIREMENTS.md) and [vocabulary](CONTEXT.md)
- [Planning maps](wayfinder/map.md): every design decision and why
- [Changelog](CHANGELOG.md)

## Development

```sh
make build                    # frontend + binary
make test                     # go vet + go test (the frontend must be built first)
make check                    # everything CI runs
bin/designer --no-open .      # then, for live frontend work:
cd web && npm run dev         # Vite on :5173, proxying /api to :4717
```

Working with an AI coding agent? Start with [AGENT.md](AGENT.md).

## Contributing

Designer is built with AI coding agents, and outside pull requests are not accepted. Bug reports and feature ideas are very welcome: see [CONTRIBUTING.md](CONTRIBUTING.md). To report a security issue, see [SECURITY.md](SECURITY.md).

## License

[Apache License 2.0](LICENSE). See [NOTICE](NOTICE) for credits and [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) for bundled software.
