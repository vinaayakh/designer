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

### Logs and performance

Designer writes JSON-lines logs to `<project>/.designer/logs/designer.log` (rotated at 20 MB, 5 kept) and readable lines to stderr.

- `--verbose` (or `--log-level debug`): every request, MCP tool call, timer, browser timing and resource sample. The default `info` logs startups, Assistant turns and every warning or error.
- `--slow-ms 500`: requests, tool calls and tasks slower than this are logged as `slow request` / `slow tool call` / `slow task` warnings. Assistant turns are slow after 3 minutes.
- `--sample 15s`: how often CPU (% of one core) and memory (RSS) of Designer and Claude Code are sampled; sustained CPU over 90% or memory over 1 GB (Designer) / 2 GB (Claude Code) is a warning. `0` turns sampling off.
- `--log-dir <dir>`: write the log somewhere else.

Each Assistant turn logs `assistant turn` with its kind, model, wall time, time to first token, tokens in/out/thinking, cache reads/writes and cost. The browser sends its own timings to the same log (`"source":"web"`): API round trips, ELK layout, routing, rendering, review build/swap, export images, long tasks (> 200 ms is a warning), dropped frames while panning, JS heap, and uncaught errors. `GET /api/metrics` returns this run's totals: per-route and per-task counts with p50/p95/max, token totals per turn kind, and the latest and peak CPU/RAM samples.

To find problems: `grep '"level":"WARN"\|"level":"ERROR"' designer.log`, or filter by `"msg":"slow request"`, `"msg":"assistant turn"`, `"msg":"dropped frames"`, `"msg":"resources"`.

`npm run bench:pan -- [url] [--throttle 4] [--dpr 2] [--rough] [--hz 120] [--no-snapshot]` (in `web/`, needs Microsoft Edge) measures frame times, dropped frames, long tasks and script time while panning with the wheel, pinch-zooming, dragging with the hand tool and panning at 1x zoom. Saving is blocked, so it is safe on a real project. Build with `npx vite build --minify false` for readable profiles.

While you pan or zoom (wheel, trackpad, hand tool, Space + drag, middle button), the canvas moves a snapshot of the last frame instead of letting Excalidraw redraw every element, and hands the final position to Excalidraw when the gesture ends (`web/src/snapshotPan.ts`). To compare with Excalidraw's own handling, run `localStorage.setItem("designer.snapshotPan", "off")` in the browser console and reload.

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
- **Documents (Document Pipeline):** the Documents tab writes a Requirements spec, a Design doc with ADRs, a Threat model (STRIDE) and a Reliability & DR plan, each built on the ones before it. Each Step asks its questions first (answers are recorded; `unknown` becomes an assumption), then the Assistant writes it, citing the **Register**: the project's append-only, hash-chained audit trail of requirements, assumptions, constraints, non-goals, decisions, risks and open questions (`register/log.jsonl`). You confirm or reject what the Assistant adds; a document goes stale when anything it relied on changes. Versions are kept as `documents/<step>/vNNN.json` with a Markdown copy beside each. When you state a requirement or constraint in chat, the Assistant proposes it as an entry for you to accept. Conversion designs against your confirmed requirements, and accepting a "Draft a fix" Proposal closes the risk it fixes. The arc42 export reads the same Register. **Download document pack** zips the documents, an ADR per decision (MADR 4.0), the Register with its history and raw log, and an index of what each document was built from. Spec: [wayfinder/doc-pipeline](wayfinder/doc-pipeline/map.md).
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
