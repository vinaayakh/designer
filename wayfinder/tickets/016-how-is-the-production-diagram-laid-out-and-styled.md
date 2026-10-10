---
id: "016"
title: "How is the Production Diagram laid out and styled?"
labels: [wayfinder:grilling]
status: closed
assignee: vinu (with claude)
blocked_by: ["015"]
parent: map
---

# How is the Production Diagram laid out and styled?

## Question

Using the findings from "How do we draw the Production Diagram?" and the numbers from "Is Excalidraw fast enough? (spike)", decide:
1. **Layout engine:** ELK.js (handles nested groups and cross-group edges; about 462 KB, loaded only when needed in a Web Worker) or dagre (about 17 KB, weak on cross-group edges)?
2. **Arrow style:** Excalidraw elbow arrows (re-route when you move things) or ELK's exact routes? And what happens to arrows when you hand-edit after layout?
3. **Generic icon set:** Tabler (MIT) or Lucide (ISC)?
4. **Placing Proposal changes:** Proposals never carry coordinates (see "How are Proposals shown and reviewed?"). Where does the runtime put new and moved Nodes: an incremental layout around their neighbours, or a full re-layout? The overlay preview must use the same positions that will be applied.
5. **Groups:** since Excalidraw frames can't be nested, groups (Region/VPC/Subnet/AZ) are drawn as rectangles. How do they look and resize, and is re-layout available on demand after manual edits?

Constraints from "Is Excalidraw fast enough? (spike)":
- ELK in a Web Worker takes about 0.28 s for 50 nodes, 1.0 s for 200 and 2.55 s for 500 (plus 0.4–1.4 s the first time a worker starts). The main thread never froze.
- **Moving shapes from code does not re-route their bound arrows.** Re-layout, and Proposals that move Nodes, must recalculate the connected arrows themselves.
- **GPU memory risk:** long arrows spanning the whole diagram are expensive (60 of them reached 2.4 GB of GPU memory at 500 elements). Layout and arrow style should keep arrows short and local.

_From "How does Conversion work?":_ Conversion's Design task returns a **whole new diagram** with no coordinates (all `add_*` ops), shown in a read-only **Conversion Preview** before Accept. Layout must handle a full first layout of up to about 300 Nodes, and the preview must use the same positions that Accept saves.

## Resolution

Decided with vinu, 2026-10-10.

**Layout engine: ELK.js (layered), loaded lazily in a Web Worker**
- **Never loaded** for sessions that use only the Rough Draft.
- **Warmed up when the Design task starts:** the AI turn takes tens of seconds, which hides both the download and the 0.4–1.4 s first-run cost.
- dagre was rejected because it handles edges that leave a group poorly, and every AWS/Azure diagram has VPC and subnet Groups.
- **Flow is left to right:** users → edge/CDN → load balancer → app → data, with external services on the far right. Groups nest as column bands.

**Production Node card**
- A fixed **200×72** rounded rectangle, with `roughness: 0` and the font **Nunito** (ships with Excalidraw, served locally).
- On the left, a 40 px icon. Next to it, the label in bold and the full Service name in small text below.
- **Data Classification:** a badge in the top-right corner (`PHI`, `PII`).
- **Category tint:** on the border only (compute / data / network / security / messaging / external).
- **Long labels:** cut off with an ellipsis, full text on hover.
- **Official or generic icon:** swapped in place, with no change to the layout.

**Edges**
- **Line style:** `sync` solid, `async` dashed, `stream` dotted.
- **Protocol:** a small label in the middle of the arrow.
- **Direction:** arrowheads show it. A two-way Edge is two arrows.
- **Routing:** **Excalidraw elbow arrows everywhere.** ELK places the Nodes and picks the side each arrow attaches to (`fixedPoint`), and Excalidraw does the routing. Dragging a Node by hand re-routes its arrows cleanly.
- When the runtime moves Nodes, it **re-creates the connected arrows from skeletons** so that Excalidraw routes them again.
  - **Build risk:** this hasn't been tested. The fallback is to compute the elbow points ourselves.
  - Accepted downside: an elbow arrow can occasionally cross a card.

**Groups**
- Plain rectangles, drawn behind their Nodes, with a group-kind icon and label in the top-left.
- **Look by group kind:**
  - region: dashed grey
  - account: solid dark
  - vpc/vnet: solid with a faint fill
  - subnet: fill by tier (public light green, private light blue)
  - az: dotted
- **Resizing:** Groups **grow automatically** when the runtime adds Nodes and **never shrink automatically**. Users can resize them freely; any mismatch shows the existing warning marker.

**First layout (Conversion Preview)**
- A full ELK layout, using **Rough Draft positions as ordering hints** (semi-interactive crossing minimisation). Things drawn left of or above something stay that way where the graph allows.
- Nodes the AI added have no hint, so ELK places them.
- Rough Draft Node IDs survive Conversion, so the hints come for free.

**Placing Proposal changes: local placement, no ELK**
- A new Node goes **inside its parent Group, next to the Node it connects to most**, in the first free slot on the side its Edges point to.
- Siblings in the way are pushed along the row, and the Group grows. A moved Node (`move_node`) is placed the same way inside its new parent.
- Nothing outside the touched Groups moves.
- The overlay preview uses exactly these positions, and Accept applies them.

**Re-layout on demand**
- **Tidy up** runs ELK over the whole diagram, using the current positions as hints. **Tidy selection** runs it inside one Group.
- These are user actions, not Proposals: each saves one new version with one Undo.

**Generic icons:** **Tabler** (MIT). Only the roughly 50 mapped SVGs ship, chosen in "What are the Component Kinds and Cloud Catalog contents?".
