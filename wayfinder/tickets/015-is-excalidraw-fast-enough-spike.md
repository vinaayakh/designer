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
