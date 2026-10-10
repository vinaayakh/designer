# 1. Diagram meaning lives in the Excalidraw scene

Date: 2026-10-10
Status: Accepted

## Context

Every diagram needs both a drawing (Excalidraw elements) and a meaning (Nodes, Edges, Groups, Component Kinds, Services) that the AI and the compliance checks reason over. The usual design keeps a separate model as the source and treats the canvas as a view of it. That means keeping the two in sync, and a model and canvas drifting apart is a common failure in diagramming tools.

## Decision

Each diagram is stored as **one `.excalidraw` file**. Meaning is attached to elements through Excalidraw's `customData`. A Node's identity is the Excalidraw `element.id`. The component graph, and the compact text format the AI reads, are **calculated** from the scene when needed (one pass, well under a millisecond at our sizes). On Production Diagrams, Group membership is an **explicit parent link** stored in `customData` and set when a Node is dropped. Any mismatch with the drawn position is flagged, never silently fixed.

## Consequences

- No sync code between a model and the canvas. Hand edits are reflected in the graph straight away.
- Diagrams stay openable in excalidraw.com. Our meaning travels with them but is ignored there.
- We depend on Excalidraw keeping `customData` through save, load and duplicate, which the research confirmed for 0.18.x. An Excalidraw version upgrade needs a regression test for this.
- Moving to a different canvas library later would mean migrating `customData` into that library's format.
- Explicit parent links can disagree with the picture. The app must detect and show this.
