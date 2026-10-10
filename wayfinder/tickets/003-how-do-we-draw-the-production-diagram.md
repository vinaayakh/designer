---
id: "003"
title: "How do we draw the Production Diagram?"
labels: [wayfinder:research]
status: closed
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

## Resolution

Full findings: branch `research/003-how-do-we-draw-the-production-diagram`, file `research/003-how-do-we-draw-the-production-diagram.md`.

- **Layout: ELK.js recommended** (0.12.0, EPL-2.0 OR GPL-3.0). Its layered algorithm lays out nested groups (Region > VPC > Subnet) in one run, with edges that cross groups, ports and right-angled routing. About 462 KB gzipped, so load it only when a Conversion runs, in a Web Worker. dagre (MIT, about 17 KB) has a known problem with edges that leave a group. Agent's own test in Node: 50 nodes about 0.3 s, 500 heavily linked nodes about 4.4 s. Needs re-measuring in a browser.
- **Excalidraw:** frames can't be nested, so groups are plain rectangles and which node sits in which group lives in our data model. Icons are SVG image elements. Arrow style is open: Excalidraw's elbow arrows (re-route on edit) or ELK's exact routes.
- **AWS icons:** the terms only allow them for "architecture diagrams" (plus whitepapers and presentations). There's no licence file, and nothing either grants or forbids putting them inside a software product.
- **Azure icons:** allowed "in architectural diagrams, training materials, or documentation". They must not be cropped, flipped, rotated or distorted, and the full service name must appear next to them, so the editor must lock official icons against those edits.
- **Keeping icons current:** a script scrapes the downloads, normalises them and diffs each release. AWS releases quarterly; Azure has no fixed schedule.
- **Pattern worth borrowing:** nodes, nested auto-sized containers and edges, with rules on what can nest inside what (Brainboard-style).
- **Decisions raised:** how the icons are shipped (in "What are the Knowledge Base licensing rules?", renamed to cover icons); ELK vs dagre, arrow style and generic icon set (new ticket "How is the Production Diagram laid out and styled?").
