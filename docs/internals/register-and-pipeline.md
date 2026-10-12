# The Register and the Document Pipeline

The vocabulary is in [CONTEXT.md](../../CONTEXT.md); every decision behind this design, with the alternatives considered, is on the [Document Pipeline map](../../wayfinder/doc-pipeline/map.md).

## The Register (`internal/register`)

One per Project, stored as `register/log.jsonl`: an **append-only event log**. Each line records one event (create, status change, supersede, link, note) with the actor (`user`, `assistant` or `runtime`), the Step and document version involved, a reason, and the SHA-256 of the previous line. Designer re-checks the chain on load and shows a warning if the file was edited outside Designer.

| Type | Holds | Fields |
|---|---|---|
| REQ | requirements | kind (goal, functional, non-functional), priority (must, should, could), acceptance, rationale |
| CON | constraints | origin |
| ASM | assumptions | assumed, if_wrong |
| NG | non-goals and out of scope | kind, reason |
| DEC | decisions (rendered as ADRs) | chose, over, because |
| RISK | risks | likelihood, impact, mitigation, owner |
| Q | open questions | answer |

Rules the code enforces:

- IDs (`ASM-012`) are assigned by the runtime and never reused.
- The Assistant's entries always start `open`. Only the user confirms or rejects.
- Text is never edited. A change supersedes the old entry with a new one, so history shows every version.
- Brief answers are synced in: a measurable target becomes a confirmed REQ, any other answer a confirmed CON, an `unknown` an open ASM.
- Entries proposed in chat (`propose_entries`) wait on a review card and are only created when the user accepts them.
- Accepting a Proposal drafted for a RISK or Q supersedes it with a confirmed DEC naming the Proposal.

## The Document Pipeline (`internal/pipeline`)

A **Step** is a YAML file built into the binary (`internal/pipeline/steps/`): its questions, the entry types it may create, the Steps it builds on, and its sections. Sections are `prose`, `list` or `table` (written by the Assistant), `entries` (filled from the Register) or `derived` (computed by the runtime with no tokens: external interfaces, the data flow diagram and STRIDE-per-element table, error budgets, a failure-mode skeleton).

Running a Step:

1. The wizard's answers become confirmed Register entries; `unknown` is remembered and the Assistant states what it assumes.
2. The Assistant gets the Brief, the diagram, the live Register, upstream documents as summaries and the derived tables, and calls `submit_step` once with its sections and new entries (`+ref` placeholders).
3. Everything is validated before anything is created: the section schema, cited IDs (unknown, rejected or superseded entries are refused), fields, links. All problems come back together.
4. The version is saved as `documents/<step>/vNNN.json` (never edited) plus a rendered `vNNN.md`, with a build record: Brief hash, Production version, upstream versions, answers time, cited entries with their status, Knowledge Base Records used.

A document's status is computed, never stored: `waiting`, `needs` answers, `ready`, `current`, or `stale` with the reasons (the Brief, diagram or an upstream document changed, an answer changed, or a cited entry was rejected or superseded). Nothing is regenerated automatically.

The **document pack** (`POST /api/documents/pack`) zips each Step's latest version as written, one MADR 4.0 ADR per DEC (`DEC-012` is `ADR-0012`), `register.md`, the raw log and an index.
