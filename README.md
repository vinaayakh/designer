# Designer

A local, AI-assisted architecture diagramming tool. Sketch a system as a hand-drawn **Rough Draft**, then convert it into a cloud-specific **Production Diagram** with a Design Rationale and a SOC 2 / HIPAA **Gap Report**. AI work runs in your local Claude Code; the app never handles API keys.

The spec is in [REQUIREMENTS.md](REQUIREMENTS.md), the vocabulary in [CONTEXT.md](CONTEXT.md), and the decisions behind it on the [planning map](wayfinder/map.md).

## Run it

Requirements: Go 1.26+, Node 22+, and [Claude Code](https://claude.com/claude-code) 2.1.295 or later, logged in.

```sh
./scripts/build.sh          # frontend + tests + bin/designer
bin/designer ~/my-system    # opens (or creates) a Project folder and the browser
```

Flags: `--port` (default 4717, falls back to a free port), `--no-open`, `--claude <path>`.

For frontend work, run `bin/designer --no-open .` and `npm run dev` in `web/` (Vite proxies `/api` to port 4717).

## How it fits together

| | |
|---|---|
| `cmd/designer` | the binary: opens the Project, serves the UI on 127.0.0.1 |
| `internal/brief` | Design Brief schema (`schema.yaml`), validation, `brief.yaml`, the compact line the AI reads |
| `internal/catalog` | 48 Component Kinds, group kinds, Data Classifications, AWS and Azure Cloud Catalogs (`data/*.yaml`) |
| `internal/kb` + `kb/records` | the Knowledge Base: Record format, lint, copy check, BM25 search, `applies_to` lookup |
| `internal/scene` | `.excalidraw` scene → graph (Nodes, Edges, Groups, Notes), Aliases, compact format, change lists, pre-check |
| `internal/proposal` | validates `submit_proposal`, resolves Aliases, dependencies between changes, simulation |
| `internal/project` | the Project folder (below) |
| `internal/agent` | the long-running `claude -p` process (stream-json) |
| `internal/app` | Assistant Session lifecycle, the 6 MCP tools, trusted-domain hook, Conversion, Gap Check, Fix |
| `internal/server` | JSON API, event stream, `/mcp`, `/hook`, embedded frontend |
| `web/` | React + Excalidraw UI: wizard, canvas, Proposal review, Conversion Preview, Rationale, Gap Report, Inspector, versions |

A Project folder holds plain files you can commit:

```
project.yaml                 name, created, when the Rough Draft was frozen
brief.yaml                   the Design Brief, every field written out
rough-draft.excalidraw
production/vNNN.excalidraw   one per Production version
production/vNNN.json         version meta + Design Rationale snapshot
gap-reports/vNNN-<time>.json never edited after they are written
.designer/                   runtime state (Assistant Session, waiting Proposal); git-ignored
```

## Tests

`go test ./...` covers the Brief, catalogs (Kind coverage, ≤1.2k-token exports), the Knowledge Base CI checks (lint, copy check, 32-query search eval), scene parsing and change lists, Proposal validation and dependencies, versions, and the trusted-source rules. `npm run check-icons` checks every Kind's Tabler icon. `npm run build` type-checks the frontend.

## Defaults taken where the map is still open

These items are still under "Not yet specified" on the map; the build picks a simple default for each:

- **Project folder layout:** as above, plus `documents/` for the arc42 document text.
- **Hand-edit versions:** edits within 10 minutes of each other are saved into one version, unless a Gap Report checked it.
- **Chat history:** not saved; the Assistant Session's memory is Claude Code's own (`--resume`).
- **Export:** PNG and SVG of the current view, and the **arc42 document** of a Production version as Word (.docx) or PDF (the browser's Save as PDF). The document is built from the Brief, diagram, Design Rationale and newest Gap Report; the Assistant writes the responsibilities, runtime scenarios, crosscutting concepts and quality scenarios once per version (`documents/vNNN-arc42.json`), or you can export without them.
- **Comparing versions:** you can view any older version read-only and restore it; there is no side-by-side diff yet.
- **Editing the Brief later:** allowed from Settings, except the target cloud after Conversion. The Assistant gets the change with the next message.
- **Usage and cost:** the session cost Claude Code reports is shown in the Assistant header.

## Not done yet

- **Official AWS/Azure icons:** the download-on-first-use flow, the terms prompt and the icon-sync script are not built. Every card uses the generic Tabler icon, which is the spec's offline fallback, and catalog `icon` keys are empty.
- **Catalog facts to verify:** the `hipaa` flags in `aws.yaml` and `azure.yaml` were written from memory; `hipaa_as_of` stays null until they are checked against the vendors' lists.
- **Install and update:** no `curl | sh` installer or `designer --update` yet.
- **Knowledge Base:** only the 5 pilot Records.
- **App licence:** not chosen.
