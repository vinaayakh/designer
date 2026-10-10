---
id: "011"
title: "How does Conversion work?"
labels: [wayfinder:grilling]
status: open
assignee:
blocked_by: ["002", "006", "007"]
parent: map
---

# How does Conversion work?

## Question

How does Conversion turn a frozen Rough Draft, the Design Brief and the Knowledge Base into the first Production Diagram? Covers: steps and prompting, the Knowledge Base-first then web-search order, how sources are cited, mapping generic shapes to catalog services, and the format and storage of the **Design Rationale** and **Gap Report** (met / unmet / outside what the diagram can show, with fix suggestions turned into Proposals).

_Note from "How is the Claude Code session configured?": the model can be switched mid-session (`set_model` control message), so Conversion could run on a stronger model in the same process. Decide here whether it should._

_Constraints from "What is the Knowledge Base record format?":_
- The Records that apply to a diagram are found by an `applies_to` lookup the runtime runs (Kinds, Data Classifications, cloud, Brief), not by search. Decide whether the runtime puts that list (IDs and summaries) into the prompt or the AI asks for it through a tool, given the ~6-tool budget.
- The AI judges met or unmet. Records marked `coverage: process` go straight to "outside what the diagram can show".
- A control's `satisfied_by` links to the practices that become fix suggestions, and the reverse `satisfies` link feeds the Design Rationale.
- Every Gap Report is stamped with the Knowledge Base version.

_From "Write the 5 pilot Knowledge Base records":_ `applies_to` matches by Kind, so practices over-apply (e.g. the S3 gateway endpoint practice matches every S3 Node on AWS, inside a VPC or not). Treat matched practices as **candidates** for the AI to judge, not findings. A control's `satisfied_by` list may be incomplete while the Knowledge Base is still being written.
