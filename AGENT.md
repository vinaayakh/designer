# Guide for AI coding agents

Designer is built with AI coding agents. This is what an agent needs to work here quickly and safely.

## Read first

- [REQUIREMENTS.md](REQUIREMENTS.md): what Designer must do, with status markers.
- [CONTEXT.md](CONTEXT.md): the vocabulary (Rough Draft, Production Diagram, Proposal, Register, Step…). Use these terms exactly, in code and prose.
- [wayfinder/](wayfinder/map.md): the planning maps. Every design decision is a closed ticket with its reasoning. Check before changing a decided behaviour; if you must, say which ticket it overrides.

## Commands

| Task | Command | Notes |
|---|---|---|
| Build the frontend | `cd web && npm run build` | Needed before any Go build or test: the UI is embedded with `go:embed` from `web/dist`. |
| Type-check the frontend | `cd web && npx tsc --noEmit` | Fast; run after frontend edits. |
| Test one package | `go test ./internal/<pkg>` | Seconds. Prefer this while iterating. |
| Vet and test everything | `make test` (or `go vet ./... && go test ./...`) | Run once when your changes are complete. |
| Everything CI runs | `make check` | Frontend, vet, tests, third-party notices. |
| Local binary | `make build` (or `scripts/build.sh` on Windows) | `bin/designer`. |
| Release archives | `make dist` | All platforms into `dist/`. |
| Third-party notices | `make notices` | After changing npm or Go dependencies; commit `THIRD_PARTY_NOTICES.md`. |

Don't run the full suite after every small edit. Finish the change, then test, then fix until it's green.

To try a change against a real project without touching the user's, copy the Project folder somewhere temporary and run a second instance on another port (`bin/designer --port 4795 --no-open <copy>`). The Assistant's calls cost real money; say so before running them.

## Layout

- `cmd/designer`: the binary. `internal/`: one package per subsystem; see [docs/internals/architecture.md](docs/internals/architecture.md) for the map.
- `web/`: React + Excalidraw (TypeScript, Vite). Panels in `web/src/panels/`, the canvas in `web/src/Canvas.tsx`, the API client in `web/src/api.ts`, types in `web/src/types.ts`.
- `kb/records`: Knowledge Base Records (Markdown + YAML front matter). Tests lint them and reject long verbatim copies from non-free sources (ADR 0002).
- `internal/pipeline/steps/*.yaml`: Document Pipeline Steps. Bump a Step's `version` when its sections or questions change, so earlier documents show as stale.
- `internal/app/prompt.md`: the Assistant's system prompt. Task-specific instructions are built in Go next to each task.

## Rules that matter

1. **Nothing changes the diagram without an approved Proposal.** The Assistant only proposes; the user accepts.
2. **The Register is append-only.** Never edit or delete entries or rewrite `register/log.jsonl`; supersede. Only the user confirms or rejects; the Assistant's entries start open.
3. **Projects are plain files that diff well.** Keep formats stable and human-readable; runtime state goes in `.designer/`.
4. **Token efficiency.** Send the Assistant compact formats and summaries, compute what can be computed (derivers) instead of asking the model, and validate submissions so every problem comes back in one round trip.
5. **The Assistant's sandbox stays closed:** Designer's MCP tools plus WebSearch/WebFetch on trusted domains only. Don't widen it.
6. **Content licensing (ADR 0002):** don't paste text from standards, cloud docs or cheat sheets into Records, prompts or templates. Write facts in our own words and cite the source.
7. **Single binary, no new runtime dependencies** for users beyond Claude Code.

## Style

- Match the surrounding code's comment density, naming and idiom. Comments say why, not what.
- User-facing text is plain, specific and short. Use the CONTEXT.md terms.
- Tests sit next to the code (`*_test.go`) and test behaviour, not implementation details.

## Documentation

When a change is complete, update what it affects:

- CLI flags → [README.md](README.md) and [docs/internals/logging-and-performance.md](docs/internals/logging-and-performance.md)
- User-visible features → [docs/features/](docs/features/README.md)
- Architecture → [docs/internals/](docs/internals/)
- Notable changes → the `Unreleased` section of [CHANGELOG.md](CHANGELOG.md)

## Commits and releases

- One logical change per commit, with a message that says what changed and why.
- Releases follow [PUBLISHING.md](PUBLISHING.md): `make release V=x.y.z` only after `CHANGELOG.md` has that version's section and `make check` passes.
