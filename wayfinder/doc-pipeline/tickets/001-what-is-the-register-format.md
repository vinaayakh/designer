---
id: "001"
title: "What is the Register format?"
labels: [wayfinder:grilling]
status: closed
assignee: vinu (with claude)
blocked_by: []
parent: map
---

# What is the Register format?

## Question

The Register is the Project's shared, append-only audit trail of requirements (REQ), assumptions (ASM), constraints (CON), non-goals and out of scope (NG), decisions (DEC) and open questions (Q). What exactly is an entry, and how is the Register stored and changed?

- **Fields:** ID scheme (e.g. `ASM-012`, stable forever), statement, type, **source** (Brief answer, wizard answer, chat, AI assumption, diagram element, Knowledge Base Record), the Step and document version that introduced it, **status** (open, confirmed, rejected, superseded), what supersedes it, and links (Nodes, Edges, Controls, other entries).
- **Append-only in practice:** is a status change a new event in a log (event-sourced, e.g. JSON Lines) or an edit to the entry with history kept by git? What does "auditable" require here?
- **Storage:** one file per entry, one file per type, or one log, in the Project folder, diffing well in git.
- **Existing data:** how today's Brief `unknown` answers, Design Rationale `assumptions` and trade-offs, and Gap Report findings become (or link to) Register entries, without two sources of truth.
- **Who changes status:** the user confirming or rejecting an assumption, a later Step superseding it, a Proposal invalidating it. What happens to documents that cited an entry that's now rejected?

## Resolution

Decided 2026-10-12. Round 1 agreed with vinu; round 2 taken as recommended (vinu: "for future queries find the recommended solution and proceed with it").

**Storage: an event log.** `register/log.jsonl` in the Project folder, one JSON event per line, only ever appended by the runtime (under its lock). The current state of every entry is computed from the log, and a readable `register.md` is rendered on export. The audit trail stands on its own without git.

**Integrity: a hash chain.** Each event carries `prev`, the SHA-256 of the previous line. The runtime checks the chain on load and reports any break, so edits outside Designer are detectable (not prevented).

**Entry types (7):**

| Type | Meaning | Type-specific fields |
|---|---|---|
| `REQ` | requirement | `kind: goal\|functional\|non-functional`, `priority: must\|should\|could`, `acceptance` (testable criterion), `rationale` |
| `ASM` | assumption | `assumed` (the value or belief), `if_wrong` (what it would affect) |
| `CON` | constraint | `origin: brief\|regulatory\|technical\|organisational` |
| `NG` | non-goal or out of scope | `kind: non-goal\|out-of-scope`, `reason` |
| `DEC` | decision | `chose`, `over` (alternatives), `because`; becomes an ADR when rendered |
| `RISK` | risk | `likelihood`, `impact` (low/medium/high), `mitigation`, `owner`; accepting one is a linked `DEC` |
| `Q` | open question | `answer` when answered (answering closes it and usually creates another entry) |

**IDs:** type prefix plus a per-Project sequence (`ASM-012`), assigned by the runtime on creation, never reused (rejected entries keep theirs). The AI never chooses IDs; it proposes entries and cites existing IDs, which the runtime validates.

**Event:** `{seq, at, actor: user|assistant|runtime, step, doc?, op, id, reason?, prev}` plus, by `op`:
- `create`: the entry (`type`, `statement`, type-specific fields, `source`, `links`, initial `status`).
- `status`: the new status (`confirmed`, `rejected`).
- `supersede`: the replacing entry's ID. An entry's text is never edited: a change is a new entry that supersedes the old one.
- `link`: links added later (e.g. a document that cites the entry, a Proposal that touched it).

**Source:** `{kind, ref}` with kind `brief` (field ID), `wizard` (Step and question ID), `chat` (the message text, kept in the event, since chat history is not otherwise stored), `assistant` (an AI assumption or inference), `diagram` (element IDs and Production version), `kb` (Record ID), `gap` (Gap Report file and Control) or `import`.

**Links:** to Nodes, Edges and Groups (element IDs), Knowledge Base Records, Gap Report items and other entries.

**Lifecycle:** `open` → `confirmed` or `rejected`; any status → `superseded` (pointing to its replacement). The Assistant only creates entries as `open`. Only the user confirms or rejects. What the user states (Brief and wizard answers, a chat instruction they confirm) is created `confirmed`.

**Existing data, one source of truth:**
- **Brief:** stays the source for its own answers; it is not copied into the log. Steps cite Brief fields directly, and the Requirements Step turns them into `REQ` and `CON` entries that link the field. An `unknown` Brief answer gets an `ASM` (the assumed value) linked to the field; answering the field later supersedes that `ASM` automatically.
- **Design Rationale:** from now on Conversion and Proposals write their assumptions as `ASM` and their trade-offs as `DEC` entries, and the Rationale keeps the IDs instead of the text. Existing Projects are imported once on first load (`source: import`, status `open`).
- **Gap Reports:** stay immutable evaluations, not entries. A Gap Report item the user decides not to fix becomes a `RISK` and an accepting `DEC`, linked to the item.

**When an entry changes:**
- A document version that cites an entry that is later rejected or superseded becomes **stale**, with that entry named as the reason. Rejecting an assumption asks for a replacement (optional), created `confirmed` and superseding it.
- When a Proposal or hand edit deletes an element an entry links to, the entry is flagged **needs review** (computed, not a status). The user confirms, rejects or supersedes it.
