---
id: "016"
title: "How is the Production Diagram laid out and styled?"
labels: [wayfinder:grilling]
status: open
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
