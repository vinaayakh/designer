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
1. ~~**System prompt:** replace or append?~~ **Decided in "How few tokens can the Assistant use? (spike)": replace** (`--system-prompt`). Still open: what the prompt says.
2. **Trusted-domain enforcement:** WebSearch can't be limited by domain. Choose between a hook in our Go server that blocks it, accepting search but limiting WebFetch, or relying on the prompt only.
3. **Process lifecycle:** ~~long-running vs per turn~~ **decided in the token spike: one long-running process per open Project.** Still open: idle shutdown, and what to do when it crashes (resuming with `--resume` keeps the cache).
4. **MCP transport:** localhost Streamable HTTP or stdio.
5. **Model choice:** a fixed default, user-selectable, or different per task (e.g. a stronger model for Conversion, a lighter one for chat)?
6. **The `--bare` risk:** what we do if `-p` starts requiring an API key.
