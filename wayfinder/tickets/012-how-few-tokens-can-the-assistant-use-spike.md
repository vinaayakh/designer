---
id: "012"
title: "How few tokens can the Assistant use? (spike)"
labels: [wayfinder:prototype]
status: open
assignee: claude (spike subagent)
blocked_by: ["002", "006"]
parent: map
---

# How few tokens can the Assistant use? (spike)

## Question

Measure with the real Claude Code (also confirm the stdin message format and how interrupting a turn works, both undocumented; see "How do we run Claude Code as a background agent?"): tokens per turn and response time for (a) raw Excalidraw JSON vs the compact form, (b) putting selected nodes and the Design Brief summary in the prompt vs fetching them with tools, (c) one long-running session with prompt caching vs a fresh process each turn, (d) a tool list of about 5 tools vs a larger one, (e) sending only what changed on later turns, (f) a compact catalog of Knowledge Base records (ID, title, tags) vs search-only access. Output: the context strategy for the Assistant, with numbers.
