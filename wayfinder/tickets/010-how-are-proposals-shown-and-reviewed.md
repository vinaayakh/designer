---
id: "010"
title: "How are Proposals shown and reviewed?"
labels: [wayfinder:prototype]
status: closed
assignee: vinu (with claude)
blocked_by: ["006"]
parent: map
---

# How are Proposals shown and reviewed?

## Question

How is a Proposal shown on the canvas and in the sidebar (ghosted additions, struck-through removals, a change list?), and how does the user accept it, reject it, or reply with changes? Can parts of a Proposal be accepted on their own? What does the data look like when a Proposal is submitted through MCP?

## Resolution

Decided with the user, 2026-10-10. **Variant D, step through one change at a time, with a jump to a full review that uses Variant A's overlay.** Prototype: branch `prototype/010-how-are-proposals-shown-and-reviewed`, file `prototypes/010-proposal-review/index.html` (opens on D).

**Showing a Proposal**
- **On the canvas**, changes are drawn as an overlay on the active diagram:
  - new Nodes and Edges are dashed green
  - removed ones are red and struck through
  - changed ones are orange, with the old value struck through
  - moved Nodes are blue, with a dashed outline where they used to be
  - a badge on each Node shows `+ − ~ ↦`
- **In the sidebar**, the Proposal's title and summary, plus a list of changes. Each change has a one-line reason (`why`) and its review status (accepted / skipped / not reviewed). Clicking a change jumps to it.

**Reviewing a Proposal**
- **Step through:** the canvas zooms to one change at a time, with a progress bar, **Accept**, **Skip** and ← →. At the end it shows a summary and **Apply N changes**.
- **Skip to full review:** shows every change not yet skipped on the whole diagram at once, with one **Accept** for all of them, plus **Reject** and **Reply**. Clicking a change goes back to stepping from that change.
- **Parts of a Proposal can be accepted on their own.** The **runtime** works out dependencies, so the AI doesn't send them: an Edge to a new Node needs that Node. Skipping a new Node skips the Edges that need it.
- **Reply with changes** sends the text to the Assistant. It answers with a new **revision** of the same Proposal; earlier revisions are marked replaced.
- **Reject** declines the whole Proposal.

**Applying a Proposal**
- **Everything accepted from one Proposal is saved as one new diagram version, and one Undo reverts all of it.**
- **The canvas is locked while a Proposal is waiting for review.** The user must accept, reject or reply before editing by hand, so a Proposal can never be out of date relative to the diagram it was based on.

**The `submit_proposal` payload** (draft; field names can change during the build)
```json
{ "title": "...", "summary": "...", "view": "production", "base_version": 3,
  "addresses": ["gap:hipaa.164-312-a-2-iv"],
  "changes": [
    {"id":"c1","op":"add_node","ref":"+waf","label":"WAF","kind":"security.waf","service":"aws.waf","parent":null,"why":"..."},
    {"id":"c2","op":"add_edge","from":"+waf","to":"n3","style":"sync","protocol":"HTTPS","why":"..."},
    {"id":"c3","op":"move_node","ref":"n1","parent":"g3","why":"..."},
    {"id":"c4","op":"update_edge","from":"n1","to":"n2","set":{"protocol":"SQL over TLS"},"why":"..."},
    {"id":"c5","op":"update_node","ref":"n2","set":{"encrypted":true},"why":"..."},
    {"id":"c8","op":"remove_node","ref":"n6","why":"..."} ] }
```
- **Operations:** `add_node`, `add_edge`, `update_node`, `update_edge`, `move_node` (changes the parent Group), `remove_node`. Removing a Node also removes its Edges.
- **References:** existing Nodes are referred to by their aliases. New Nodes get temporary `+name` references that other changes can use. Edges are identified by `from`/`to`.
- **No coordinates from the AI.** The runtime places new and moved Nodes, which ties into "How is the Production Diagram laid out and styled?". `addresses` links the Proposal to the Gap Report items it fixes.
- **What goes back to the Assistant** on its next turn, as part of the list of changes:
  - after a decision: `{proposal, revision, status: accepted|partially_accepted|rejected, applied:[ids], declined:[ids], new_version}`
  - after a reply: `{status: changes_requested, reply}`
