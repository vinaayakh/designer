---
id: "003"
title: "How do we draw the Production Diagram?"
labels: [wayfinder:research]
status: open
assignee: claude (research subagent)
blocked_by: []
parent: map
---

# How do we draw the Production Diagram?

## Question

How is a Production Diagram drawn cleanly and laid out automatically, using official AWS and Azure icons?

Find out:
- Automatic layout options in the browser (ELK.js, dagre, others): quality for architecture diagrams (groups such as VPC/subnet/region, ports, edge routing), speed, bundle size.
- Placing the laid-out result on an Excalidraw canvas (or whatever ticket "Can Excalidraw be our canvas?" recommends) so the user can keep editing it.
- Official icon sets: AWS Architecture Icons and Azure Architecture Icons. Licence and terms for shipping them inside an app, file formats, and how to keep them current.
- Generic (provider-neutral) icons: an icon set with a suitable licence.
- How established tools (draw.io, Cloudcraft, Eraser, Brainboard, the Python `diagrams` library) handle grouping and layout. Patterns worth borrowing.

## Context

Findings: branch `research/003-how-do-we-draw-the-production-diagram`, file `research/003-how-do-we-draw-the-production-diagram.md`.
