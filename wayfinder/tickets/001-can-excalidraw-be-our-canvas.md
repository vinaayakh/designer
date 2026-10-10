---
id: "001"
title: "Can Excalidraw be our canvas?"
labels: [wayfinder:research]
status: closed
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

## Resolution

Full findings: branch `research/001-can-excalidraw-be-our-canvas`, file `research/001-can-excalidraw-be-our-canvas.md`.

**Recommended: use `@excalidraw/excalidraw` (MIT, 0.18.1) for both the Rough Draft and the Production Diagram.** Every capability question is answered yes by a public API. Speed is likely fine but not yet measured.
- **Custom data:** `customData` on each element survives saving and reloading. Copy, paste and duplicate copy it onto an element with a new `id`; fix up in `props.onDuplicate`.
- **Selection:** `appState.selectedElementIds` arrives on every `onChange`; `onPointerDown`/`onPointerUp` report the clicked element.
- **Changes from code:** `updateScene(..., captureUpdate: CaptureUpdateAction.IMMEDIATELY)`, so one undo reverts a whole Proposal. `convertToExcalidrawElements` creates shapes and arrows (labelled beta).
- **Bundle:** about 337 KB gzipped of Excalidraw's own code and CSS up front (about 350–420 KB with dependencies) plus React. Mermaid, fonts and locales load only when needed. **Fonts must ship locally** (`EXCALIDRAW_ASSET_PATH`), since the default loads them from a CDN.
- **Large diagrams:** no published benchmark. Users report slowdowns at about 5,000–8,000 elements, roughly 10× our 500 target. Not measured yet.
- **Clean look:** `roughness: 0` plus Nunito/Helvetica, set per element.
- **Upkeep:** active repo but rare stable releases (0.18.0 Mar 2025; 0.18.1 Apr 2026 was a security patch only). React 17–19, ESM-only; Vite may need `build.target: "es2022"`.
- **Alternatives are worse:** tldraw needs a paid production licence key; React Flow can't do freehand sketching.

**Decisions raised:** where the stable node ID lives (in "What is the diagram data model?"); version pin and speed check (new ticket "Is Excalidraw fast enough? (spike)").
