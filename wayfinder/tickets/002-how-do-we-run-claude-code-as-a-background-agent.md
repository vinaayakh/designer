---
id: "002"
title: "How do we run Claude Code as a background agent?"
labels: [wayfinder:research]
status: open
assignee: claude (research subagent)
blocked_by: []
parent: map
---

# How do we run Claude Code as a background agent?

## Question

How does the Go runtime drive a local Claude Code install as the Assistant, without the app ever handling API keys?

Find out:
- Headless mode: `claude -p`, `--output-format stream-json`, `--input-format stream-json`. Can one long-running session take messages one after another? How are sessions resumed (`--resume` / `--session-id`)?
- Connecting it to an MCP server run by our Go process (`--mcp-config`, stdio vs HTTP transport) and limiting it to our tools plus web search (`--allowedTools`, `--disallowedTools`, permission modes). Can web search be limited to an allowlist of domains?
- Setting the system prompt (`--append-system-prompt` / `--system-prompt`) so it uses the Knowledge Base before the web.
- Prompt caching across turns in one session, and what can be learned about tokens used and cost from the stream output.
- Startup time, memory, and how to handle failures (not installed, not logged in, rate limited).
- How px0 (https://github.com/px0-ai/px0) launches and talks to Claude Code.
- The official Go MCP SDK (`modelcontextprotocol/go-sdk`): how mature it is and which transports it supports.

## Context

Findings: branch `research/002-how-do-we-run-claude-code-as-a-background-agent`, file `research/002-how-do-we-run-claude-code-as-a-background-agent.md`.
