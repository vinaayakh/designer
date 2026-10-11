---
id: "002"
title: "What is a Step definition?"
labels: [wayfinder:grilling]
status: closed
assignee: vinu (with claude)
blocked_by: ["001"]
parent: map
---

# What is a Step definition?

## Question

Each document is produced by a Step that builds on earlier outputs. What does a Step declare, and how does the runtime run it?

- **Inputs:** which Brief fields, which diagram (Rough Draft, a Production version), which earlier documents, which Register entry types.
- **Required information:** the facts the document can't be written without, each with where it can come from (Register, Brief, diagram, earlier document) and the wizard question to ask if none has it.
- **Output:** the document's section schema (what the AI submits, validated like `submit_document`), and which Register entries the Step may add (e.g. a Threat model adds DEC and ASM, never REQ?).
- **Order:** the dependency graph between Steps. In particular, does the Requirements spec run **before Conversion** and feed Design, or after it?
- **Staleness:** what makes a document version stale (an input version changes, a cited Register entry is rejected or superseded) and what the user sees.
- **Where Steps live:** built-in definitions (like `internal/brief/schema.yaml`) so new documents need no code, or code per Step.
- **Running:** one Assistant task per Step (like the arc42 text), its submit tool, and how it reuses `internal/report` rendering.

## Resolution

Decided 2026-10-12, taking the recommended option throughout (vinu: "find the recommended solution and proceed with it").

**Steps are data, not code.** Each Step is a YAML file built into the binary (`internal/pipeline/steps/<id>.yaml`), checked against a schema at load, like `internal/brief/schema.yaml`. A new document needs a new file, not new code. The only code per document is a small set of named **derivers**: runtime functions that compute facts locally, without tokens (e.g. the data flow diagram and boundary crossings, the STRIDE-per-element table, composite availability). A Step lists the derivers it uses.

**A Step declares:**

| Field | What it holds |
|---|---|
| `id`, `title`, `version` | Stable ID (`requirements`, `design-doc`, `threat-model`, `reliability`, `arc42`) and a definition version, bumped when its structure changes |
| `template` | The structure it follows and its credit line (from research 008) |
| `inputs` | Brief fields (or `all`), diagram (`rough` or `production`), upstream Steps (`requires` must exist; `uses` if present), Register entry types it reads |
| `needs` | The facts it can't be written without. Each has an ID, where to look first (Brief field, Register filter, diagram query, upstream section), the wizard question if none has it, and whether `unknown` is allowed (default yes, then it becomes an `ASM`) |
| `derive` | Derivers to run; their results go into the prompt and some straight into sections (tables the AI only annotates) |
| `sections` | The output schema: ID, title, kind (`prose`, `table` with columns, `list`, `entries` (rendered from Register entries), `derived`), and limits |
| `creates` | Entry types the Step may propose (e.g. the Threat model proposes `RISK`, `DEC`, `ASM`, `Q`, never `REQ`) |

**Order** is a graph:

```
Brief ─▶ requirements ─▶ (Conversion reads confirmed REQ / CON / NG if present)
Production version ─┬▶ design-doc   (requires requirements)
                    ├▶ threat-model (uses requirements)
                    ├▶ reliability  (requires requirements: its SLOs follow user journeys)
                    └▶ arc42        (uses all of the above)
```

The Requirements spec runs **before Conversion when it can**: it needs only the Brief and the Rough Draft. It isn't required, so existing Projects and quick sketches still convert. Without it, Conversion's assumptions go into the Register as `ASM` entries as usual.

**A document version records** what it was built from: the Step definition version, the Brief fields it read (with a hash of their answers), the diagram version, the upstream document versions, every Register entry it cites and the entry's status at that moment, the model, and the time.

**Stale** means any of those changed: a newer Production version, a Brief answer it read, a newer upstream document, a cited entry rejected or superseded, or a new Step definition version. The UI shows the reason ("built on Production v4; v6 is current", "ASM-012 was rejected"). Nothing regenerates by itself.

**Running a Step:** a pre-check finds missing `needs` (ticket 003). Then one Assistant task runs per Step, with one generic tool for all of them, which keeps the tool list short:
- **`submit_step`** takes `{step, sections, entries, cites}`:
  - `sections` must match the schema;
  - `entries` are new entries the AI proposes, created `open` with runtime IDs;
  - `cites` are existing IDs, validated.
- **Message context:** the runtime builds it from the Step's inputs. That means the compact diagram, the Register entries that match the Step's filters, the derived facts, and **summaries** of upstream documents (their section outlines and cited entry IDs, not their full text). This keeps tokens down.
- **Rendering:** reuses `internal/report` (ticket 009).
