---
id: "016"
title: "How is the Production Diagram laid out and styled?"
labels: [wayfinder:grilling]
status: open
assignee:
blocked_by: ["015"]
parent: map
---

# How is the Production Diagram laid out and styled?

## Question

Using the findings from "How do we draw the Production Diagram?" and the numbers from "Is Excalidraw fast enough? (spike)", decide:
1. **Layout engine:** ELK.js (handles nested groups and cross-group edges; about 462 KB, loaded only when needed in a Web Worker) or dagre (about 17 KB, weak on cross-group edges)?
2. **Arrow style:** Excalidraw elbow arrows (re-route when you move things) or ELK's exact routes? And what happens to arrows when you hand-edit after layout?
3. **Generic icon set:** Tabler (MIT) or Lucide (ISC)?
4. **Groups:** since Excalidraw frames can't be nested, groups (Region/VPC/Subnet/AZ) are drawn as rectangles. How do they look and resize, and is re-layout available on demand after manual edits?
