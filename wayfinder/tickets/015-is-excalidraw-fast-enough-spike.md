---
id: "015"
title: "Is Excalidraw fast enough? (spike)"
labels: [wayfinder:task]
status: open
assignee: claude (spike subagent)
blocked_by: []
parent: map
---

# Is Excalidraw fast enough? (spike)

## Question

Before locking Excalidraw into the spec, measure it. Build a throwaway Vite + React page embedding `@excalidraw/excalidraw` with fonts served locally (`EXCALIDRAW_ASSET_PATH`), and record:
- Load: bundle size (gzipped) and time to first interactive load, cold and warm.
- ELK.js layout time in a browser Web Worker at 50 / 200 / 500 nodes with nested groups (the research measured only in Node: about 0.3 s and 4.4 s).
- Speed at 500 / 2,000 / 5,000 elements (with `customData` and arrows): pan/zoom frame rate, selection response, time for `updateScene` to apply a 50-element Proposal, and memory.
- **Version pin:** compare stable `0.18.1` with a recent `@next` snapshot on the same measurements, and note any API differences that matter to us.

Output: the numbers, and a go/no-go plus a version recommendation for you to approve.

## Findings (awaiting user approval)

Code, numbers and raw results: branch `spike/015-is-excalidraw-fast-enough`, file `spikes/015-excalidraw/RESULTS.md`. Test machine: Ryzen 5 2600, 16 GB RAM, RX 6700, Edge 155, 1600×900 at DPR 1; median of 3 runs.

**Recommendation: go.**

| | 0.18.1 (stable) | @next 0.18.0-143b5b6 |
|---|---|---|
| Eager bundle (gzipped) | 426 KB (about 66 KB is React) | 746 KB as is; **567 KB with a 4-line lazy shim for mermaid** |
| Load to interactive, cold / warm (localhost) | 408 ms / 92–123 ms | 625 ms (397 ms with the shim) / similar |
| 500 and 2,000 elements | 60 fps pan; selection paints in 22–46 ms; a 50-element Proposal paints in ≤24 ms and one Undo reverts it; heap <35 MB | same |
| 5,000 elements | zoom-to-fit 30 fps; selection 118 ms | zoom-to-fit 47 fps; selection 75 ms |

- **ELK in a Web Worker** (warm): 50 nodes take 280 ms, 200 take 1.0 s, 500 take 2.55 s. The first run on a new worker adds 0.4–1.4 s. The main thread never lost a frame.
- **Version recommendation:** @next 0.18.0-143b5b6 with the mermaid shim. It is better at 5,000 elements, and its `onDuplicate` passes `origIdToDuplicateId` and can cancel a duplicate. The cost is breaking renames: `excalidrawAPI` becomes `onExcalidrawAPI`, and `scrollToContent` is replaced by `setViewport`. Stable 0.18.1 is the fallback, and it is also fast enough.
- **Confirmed on both versions:** `customData` survives on skeleton shapes and arrows; arrows bind to existing elements; elbow arrows keep `fixedPoint` and `customData`; duplicates keep `customData` under a new id.
- **Failed on both versions:**
  - **In view mode, clicking selects nothing** and `onPointerDown` isn't called (setting the selection from code does work). The frozen Rough Draft needs another way to click Nodes into the AI's context.
  - **Moving a shape from code does not re-route its arrows**, so re-layout code must recompute them.
- **Memory risk:** each element is cached as its own bitmap on the GPU. 60 arrows spanning the whole diagram pushed the GPU process to 2.4 GB at only 500 elements. With short arrows it stays at 0.3–0.9 GB.
- **Not measured:** laptop GPUs, HiDPI screens, Firefox and Safari, dragging or resizing in large scenes, image icons, the sketchy style (`roughness: 1`), a disk-cold load.
