---
id: "001"
title: "Can Excalidraw be our canvas?"
labels: [wayfinder:research]
status: open
assignee: claude (research subagent)
blocked_by: []
parent: map
---

# Can Excalidraw be our canvas?

## Question

Can the embedded Excalidraw library (`@excalidraw/excalidraw`) be the canvas for both the Rough Draft and the Production Diagram, while meeting our speed requirement?

Find out:
- Can each element carry custom data (component type, catalog ID, stable node ID) that survives saving and reloading?
- Can the app tell which element was clicked or selected, so the node can be added to the Assistant's message context?
- Can changes be applied from code (add, update, delete, connect elements) to apply an accepted Proposal, ideally with undo?
- Bundle size and time to first interactive load (lazy-loading fonts and the library), and speed on large diagrams (500+ elements). Known performance issues.
- Can the hand-drawn look be turned off (`roughness: 0`, a clean font) for the Production Diagram?
- Licence, how actively it's maintained, and whether its React version fits a Vite build.
- If any of these fail: alternatives (tldraw and its licence, React Flow / xyflow, a custom canvas), compared on the same points.

## Context

Findings: branch `research/001-can-excalidraw-be-our-canvas`, file `research/001-can-excalidraw-be-our-canvas.md`.
