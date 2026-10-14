# Documents and the Register

The Documents tab writes the documents a senior engineer is expected to produce, each building on the ones before it:

| Document | Built on | Structure |
|---|---|---|
| Requirements spec | the Brief and the Rough Draft | ISO/IEC/IEEE 29148 outline |
| Design doc + ADRs | the Requirements spec and the Production Diagram | design doc / RFC outline; ADRs in MADR 4.0 |
| Threat model (STRIDE) | the Production Diagram | the Threat Modeling Manifesto's four questions, STRIDE per element |
| Reliability & DR | the Production Diagram and the Requirements spec | SLOs and error budgets, failure modes, backup and recovery, a NIST SP 800-34 style contingency plan |
| arc42 architecture document | everything above | the arc42 template, exported as Word or PDF |

## Writing a document

The Documents tab lists every document with its status: **Not written**, **vN · up to date**, **vN · out of date** (with the reasons), **Writing…** or **Queued**.

Some documents depend on others. A document that can't be created yet is **greyed out**, with a message saying what it needs:
- **Another document.** The Design doc and Reliability & DR build on the Requirements spec, so they say "Needs the Requirements spec first" and offer **Create the Requirements spec**. The Requirements spec, in turn, says which documents it's needed by.
- **The Production Diagram.** Documents built from it say "Needs a Production Diagram: convert the Rough Draft first".
- **Optional inputs don't block.** The Threat model uses the Requirements spec if it exists, and says so, but you can create it either way.

You can still prepare a greyed-out document's answers with **Edit answers**.

1. **Create…** a document. If it still needs information, a wizard opens and asks each question on its own screen. **I don't know** is allowed for all but a few required questions; the document then states what it assumes, as an open assumption.
2. Check the **review** screen: every answer at a glance. Click one to change it.
3. **Finish**. All the answers are recorded in the Register at once, then the Assistant writes the document.
   - The Assistant cites Register entries by ID for its claims.
   - It adds new entries (assumptions, decisions, risks, open questions) as **open**, for you to review.
   - Some sections are computed without AI: the data flow diagram and STRIDE table, error budgets, a failure-mode skeleton.
4. **Read** the result. Every version is kept; older ones are under **Older…**.

The other buttons on a document:
- **Create** writes the document straight away when it needs no more answers.
- **Edit answers** opens the wizard with all of a document's questions. Change them, then **Save answers** (or update the document).
- **Update** rewrites an out-of-date document; **Rewrite** writes a current one again.

![Reading a document: its build record, the Register entries it cites, and the computed data flow diagram](../images/document-reader.png)

A document goes **out of date** when the Brief, the diagram, an earlier document or an answer changes, or when an entry it cites is rejected or superseded. It is never rewritten automatically: you choose when to **Update** it.

## One source of truth

Your answers aren't kept inside one document. They become Register entries (requirements, constraints, non-goals and so on), so every document cites the same facts.

The Assistant gets them too when it changes the diagram. It respects confirmed entries and names them in a change's reason ("meets REQ-004"). Change a fact once, and every document that cites it shows as out of date.

## The Register

The Register tab is the audit trail: every requirement, constraint, assumption, non-goal, decision, risk and open question, with where it came from (the Brief, a wizard answer, chat, the Assistant, the Design Rationale), which documents cite it, and its full history.

![The Register tab, with an entry proposed from chat waiting for review above the requirements](../images/register.png)

- **Confirm** or **Reject…** what the Assistant added. Rejecting can replace the entry with your own wording. Only you can confirm or reject.
- **Draft a fix** on a risk or open question asks the Assistant for a Proposal that resolves it. Accepting that Proposal closes the entry with a decision that names the Proposal.
- Hovering an entry highlights the diagram elements it's about.
- Entries are never edited, only superseded, and the file is hash-chained: if it's edited outside Designer, the Documents tab says so.

## From chat

When you state a requirement or constraint in chat ("data must stay in the EU", "no mobile app in v1"), the Assistant proposes it as a Register entry. A review card appears in the Assistant tab: **Accept**, **Edit** or **Reject**. Nothing is recorded until you accept, and the entry keeps the message it came from.
