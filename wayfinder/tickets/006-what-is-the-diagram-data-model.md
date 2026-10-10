---
id: "006"
title: "What is the diagram data model?"
labels: [wayfinder:grilling]
status: closed
assignee: vinu (with claude)
blocked_by: ["001", "003"]
parent: map
---

# What is the diagram data model?

## Question

What is the data model shared by the Rough Draft and the Production Diagram: nodes, edges, groups (region/VPC/subnet/zone), component types, how they map to the Generic/AWS/Azure catalogs, and the **compact form** the AI reads? And how does it relate to the Excalidraw scene (stored inside it, or alongside it)?

Also decide: does a Node's **stable ID** live in Excalidraw's `element.id`, or in our own ID inside `customData` that we reassign when an element is duplicated? (See "Can Excalidraw be our canvas?")

## Resolution

Decided with the user, 2026-10-10. ADR: [0001 Diagram meaning lives in the Excalidraw scene](../../docs/adr/0001-diagram-semantics-live-in-excalidraw-scene.md).

**Storage**
- One `.excalidraw` file per diagram is the only stored source. A Node's meaning lives in Excalidraw's `customData` on that element.
- The component graph is **calculated from the scene** whenever it's needed, in a single fast pass. There is no separate model file to keep in sync, and the files still open on excalidraw.com.

**What counts as what (Rough Draft)**

| On the canvas | Becomes |
|---|---|
| A shape (rectangle, ellipse, diamond, image) | **Node** |
| An arrow between two Nodes | **Edge** |
| An arrow not connected at both ends | **Note** (marked as a loose arrow) |
| Free text not inside a shape | **Note**, passed to the AI as an annotation |
| A shape that fully contains at least one Node | **Group** |
| Freehand lines and doodles | ignored |

- Rough Draft Nodes **don't need a type**. The label is the meaning. You can optionally tag one from a quick picker.

**Types**
- Two layers:
  - **Component Kind**: about 40 fixed, provider-neutral kinds, e.g. `data.relational`, `messaging.queue`.
  - **Service**: a Cloud Catalog entry that maps to exactly one Kind, e.g. `aws.rds-postgres → data.relational`.
- A Production Node always has a Kind. On AWS or Azure it also has a Service; on the Generic target it has only the Kind.
- Knowledge Base Records can attach to Kinds.

**Edges**
- Rough Draft Edges have direction and an optional label.
- Production Edges add an interaction style (`sync` / `async` / `stream`) and an optional protocol.

**Data Classification** (PHI / PII / public…) goes on **Nodes that store data**, not on Edges.

**Node identity**
- A Node's ID is **Excalidraw's `element.id`**. `customData` never holds an ID.
- Duplicating a Node creates a new Node, which is the intended behaviour.
- Edges point at Nodes through Excalidraw's own arrow bindings (`startBinding` / `endBinding.elementId`).

**What a Production Node is made of**
- One rounded rectangle is the Node: it carries the `customData` and has the label attached as text.
- The service icon is a separate image element, grouped with the rectangle (Excalidraw `groupIds`). Its `customData` points back to the Node's ID.
- Clicking any part selects the Node as one item.

**Group membership**
- **Production Diagram: an explicit parent link** in `customData`.
  - It is set when a Node is dropped into a Group, and cleared when the Node is dragged out.
  - Proposals and Conversion set it directly.
  - Resizing a Group or re-laying out never changes membership.
  - When the stored parent and the drawn position disagree, a warning marker appears with one-click fixes ("move into Group" / "update parent").
  - Groups carry a group kind: region / vpc / vnet / subnet / az / account.
- **Rough Draft: membership worked out from position.** A Node belongs to the smallest shape that fully contains it. A Node partly overlapping a Group counts as outside and is flagged to the AI.
  - At Conversion, this worked-out membership becomes explicit links, and ambiguous cases become questions from the AI.

**Compact format for the AI**
- A line-based text format with no coordinates, styling or Excalidraw internals. Indentation shows Group nesting.
- **Short aliases** (`n1`, `g2`) stand in for element IDs. The alias↔ID mapping stays fixed for a whole Assistant session, so prompt caching and change-only updates keep working. Proposals use aliases and the runtime translates them back.
- Notes are placed with `near <alias>`, next to the nearest Node.

Example:

```
target: aws   brief: HIPAA, AP, 50k users, read-heavy
g1 region "us-east-1"
  g2 vpc "Prod VPC"
    g3 subnet:private "App"
      n1 "Orders API" compute.container aws.ecs-fargate
      n2 "Orders DB" data.relational aws.rds-postgres [PHI]
    n3 "ALB" network.load-balancer aws.alb
n3 -> n1 sync HTTPS
n1 -> n2 sync SQL
note "~10k rps peak" near n3
```

Token cost vs raw JSON is to be measured in "How few tokens can the Assistant use? (spike)".

New ticket from this decision: "What are the Component Kinds and Cloud Catalog contents?" (it replaces the "Cloud service catalogs" item under Not yet specified).
