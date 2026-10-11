---
id: "003"
title: "How are gaps found and asked?"
labels: [wayfinder:grilling]
status: closed
assignee: vinu (with claude)
blocked_by: ["002"]
parent: map
---

# How are gaps found and asked?

## Question

Settled: wizard first, then chat, then Proposals for diagram gaps. How does that work in detail?

- **Finding gaps:** the runtime checks a Step's required information against the Register, Brief, diagram and earlier documents before any AI call (no tokens), like the Conversion pre-check. Which gaps does the AI find only while writing, and how does it raise them (a new tool such as `ask_user`, or a Q entry)?
- **The wizard:** question format (reuse the Brief schema's fields and values?), one screen per question or one form, `unknown` always allowed. An `unknown` answer becomes an ASM entry: who writes the assumption, the user or the AI?
- **Chat follow-up:** how an answer given in chat becomes a Register entry (the AI proposes the entry and the user confirms it?), so chat never silently changes the audit trail.
- **Diagram gaps:** when the right fix is a diagram change (e.g. no backup component for an RPO target), the Step drafts a Proposal; does the document wait for it or proceed with an ASM?
- **Blocking versus proceeding:** which gaps stop a Step and which become assumptions.

## Resolution

Decided 2026-10-12, taking the recommended option throughout (vinu: "find the recommended solution and proceed with it").

**1. Pre-check (no AI, no tokens).** Before a Step runs, the runtime tries to answer each `need` from its sources, in the order the Step lists them: Brief field, Register entry, diagram query, upstream section. Whatever is left becomes the Step's wizard.

**2. The wizard.** One short form per Step, reusing the Brief wizard's look and its question format (ID, label, question, help, fixed values or free text, multi-select). Every question allows `unknown`.
- **An answer becomes a Register entry right away**, `confirmed`, `source: wizard` (Step and question ID). The `need` says which type: the problem statement and goals become `REQ` entries (kind `goal`, added to the `REQ` kinds), non-goals become `NG`, threat actors and external SLAs become `CON`.
- **`unknown`** is remembered (not asked again until the user reopens it). The Assistant then writes an `ASM` for it while generating the document; it stays `open` until the user confirms it.
- **Blocking needs** are few and marked in the Step (e.g. the Requirements spec can't be written without a problem statement and at least one user role). Every other need proceeds with an assumption.

**3. While writing, no live questions.** The Assistant doesn't stop to ask mid-task (that would need a second turn and more tokens). Anything it finds missing goes into its submission as `Q` entries (open questions), shown in the document's Open questions section.

**4. Answering later, in chat or the Register view.**
- **Answering a `Q`** closes it and creates the entry the answer implies, `confirmed`. The documents citing that `Q` go stale.
- **In normal chat,** the Assistant can propose Register entries with a new tool, `propose_entries`. They appear as a small review card, like a Proposal, and are only created when the user accepts them (`source: chat`, keeping the message text). Chat never changes the audit trail silently.

**5. Gaps in the diagram.** When a document finds the design can't meet something (no backup for an RPO target, no mitigation for a threat), it records a `RISK` (or a `Q`) linked to the elements involved. The document is still written. The user can click **Draft a fix** on the entry, which starts a Proposal task (as Fix does for Gap Reports). When that Proposal is accepted, the `RISK` is superseded or closed and the document goes stale.
