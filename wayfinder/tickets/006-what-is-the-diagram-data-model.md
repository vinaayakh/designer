---
id: "006"
title: "What is the diagram data model?"
labels: [wayfinder:grilling]
status: open
assignee:
blocked_by: ["001", "003"]
parent: map
---

# What is the diagram data model?

## Question

What is the data model shared by the Rough Draft and the Production Diagram: nodes, edges, groups (region/VPC/subnet/zone), component types, how they map to the Generic/AWS/Azure catalogs, and the **compact form** the AI reads? And how does it relate to the Excalidraw scene (stored inside it, or alongside it)?

Also decide: does a Node's **stable ID** live in Excalidraw's `element.id`, or in our own ID inside `customData` that we reassign when an element is duplicated? (See "Can Excalidraw be our canvas?")
