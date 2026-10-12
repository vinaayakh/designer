# Architecture

Designer is one Go binary. It opens a Project folder, serves the React UI (embedded with `go:embed`) on 127.0.0.1, and runs **Claude Code** as a long-lived background process for everything AI. Claude Code talks back to Designer only through Designer's MCP tools.

```
browser (React + Excalidraw)  <-- JSON API + event stream -->  designer (Go)  <-- /mcp, /hook -->  claude -p (stream-json)
                                                                    |
                                                              Project folder (plain files)
```

## Packages

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
| `internal/register` | the Register: append-only, SHA-256 hash-chained event log of REQ, CON, ASM, NG, DEC, RISK and Q entries |
| `internal/pipeline` | the Document Pipeline: Step definitions (`steps/*.yaml`), answers, derivers (data flow diagram, STRIDE, error budget, failure modes), versions, staleness, Markdown, the document pack |
| `internal/report` | the arc42 document: Word (.docx) and print-ready HTML |
| `internal/obs` | logging, timers, metrics and resource sampling |

## How the Assistant is run

- Claude Code is started with `-p --input-format stream-json --output-format stream-json`, its own system prompt, `--strict-mcp-config` (only Designer's MCP server), `--setting-sources ""` (no user or project settings) and `--permission-mode dontAsk`.
- Its only tools are Designer's MCP tools (`mcp__designer__*`), `WebSearch` and `WebFetch`. It has no file or shell access.
- A `PreToolUse` hook sends every `WebSearch` and `WebFetch` call to Designer, which allows only a list of trusted domains (`internal/app/trusted.go`). If Designer can't be reached, the call is blocked (it fails closed).
- `/mcp` and `/hook` need a bearer token generated at start-up.
- A task (Conversion, Gap Check, a document Step) ends only when the AI calls that task's `submit_*` tool. Submissions are validated, and every problem is returned at once so it can fix them in one resubmission.

## Tests

`go test ./...` covers the Brief, catalogs (Kind coverage, ≤1.2k-token exports), the Knowledge Base CI checks (lint, copy check, 32-query search eval), scene parsing and change lists, Proposal validation and dependencies, versions, and the trusted-source rules. `npm run check-icons` checks every Kind's Tabler icon. `npm run build` type-checks the frontend.

## Decisions

Architecture decisions are in [docs/adr](../adr/) and, with their reasoning, on the [planning maps](../../wayfinder/map.md).
