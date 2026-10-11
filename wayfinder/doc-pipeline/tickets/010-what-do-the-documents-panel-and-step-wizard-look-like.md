---
id: "010"
title: "What do the Documents panel and Step wizard look like?"
labels: [wayfinder:prototype]
status: closed
assignee: vinu (with claude)
blocked_by: ["001", "002", "003"]
parent: map
---

# What do the Documents panel and Step wizard look like?

## Question

How does a user move through the pipeline? Build a rough, throwaway prototype to react to:

- **Documents panel:** each Step with its status (not started, needs answers, ready to generate, current, stale), what it depends on, and its latest version.
- **Step wizard:** the questions a Step needs, with the Brief always visible (as in the Brief wizard), `unknown` allowed, and what each answer will become in the Register.
- **Register view:** entries by type with status and source; confirm or reject an assumption; see which documents cite an entry and which would go stale.
- **Generation:** progress while the Assistant writes, chat questions it raises, and the result with its cited entries.

## Resolution

Decided by vinu, 2026-10-12: **variant A, the sidebar stepper.**

The prototype (three variants mounted inside the real Workspace with real Brief, diagram and Rationale, in-memory Register) is kept on branch `prototype/010-documents-panel`. Run it there with `npm run prototype` in `web/` against a running Designer.

| Variant | Shape | Verdict |
|---|---|---|
| **A: Sidebar stepper** | A Documents tab in the right sidebar: Steps as a vertical stepper with status chips; expanding one shows its wizard one question at a time, then Write it / Read; a Register tab with entries grouped by type, Confirm / Reject (with an optional replacement) / Draft a fix, and which documents cite each entry. The canvas stays as it is. | **Chosen.** It fits today's workflow (beside the diagram, like the Assistant and Gap Report) |
| B: Documents workspace | Replaces the canvas with pipeline graph, document and needs columns | Not chosen |
| C: Register ledger | A drawer under the canvas: entries × documents, rows highlight the diagram | Not chosen |

**What to carry over from the prototype into the build:**
- The tab opens on the Documents view and sits first while it's the focus; the sidebar tab bar must never hide tabs (fixed in `a017ab2`).
- One question per screen with progress dots; `Unknown` is a button next to Save, except on blocking questions; each question says what it becomes in the Register.
- Status chips: waits on, questions to answer, ready to write, writing (with phases), current (vN), stale (with every reason listed).
- Entries show ID, status, statement, source and "cited by"; rejecting asks for an optional replacement that supersedes it.
