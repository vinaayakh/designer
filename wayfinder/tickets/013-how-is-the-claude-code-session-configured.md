---
id: "013"
title: "How is the Claude Code session configured?"
labels: [wayfinder:grilling]
status: open
assignee:
blocked_by: []
parent: map
---

# How is the Claude Code session configured?

## Question

Using the findings from "How do we run Claude Code as a background agent?", decide:
1. **System prompt:** replace Claude Code's default (`--system-prompt`) or add to it (`--append-system-prompt`)? It is fixed for the life of a conversation.
2. **Trusted-domain enforcement:** WebSearch can't be limited by domain. Choose between a hook in our Go server that blocks it, accepting search but limiting WebFetch, or relying on the prompt only.
3. **Process lifecycle:** one long-running process per open Project, or a new process per turn (the px0 approach, `--resume`)? Also: idle shutdown and what to do when it crashes.
4. **MCP transport:** localhost Streamable HTTP or stdio.
5. **Model choice:** a fixed default, user-selectable, or different per task (e.g. a stronger model for Conversion, a lighter one for chat)?
6. **The `--bare` risk:** what we do if `-p` starts requiring an API key.
