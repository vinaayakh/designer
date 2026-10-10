---
id: "002"
title: "How do we run Claude Code as a background agent?"
labels: [wayfinder:research]
status: closed
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

## Resolution

Full findings: branch `research/002-how-do-we-run-claude-code-as-a-background-agent`, file `research/002-how-do-we-run-claude-code-as-a-background-agent.md`.

- **Feasible.** One long-running `claude -p` process per open Project, talking line-by-line JSON over stdin/stdout (`--input-format stream-json --output-format stream-json --verbose`). Tested locally: two turns ran in one process, and the second turn read about 3.2k tokens from the prompt cache (subscription logins get a 1-hour cache). Each turn's `result` line reports tokens used and an estimated cost. Startup about 4 s on Windows; about 180 MB while idle.
- **Tools:** our Go MCP server over localhost HTTP (`--mcp-config` with `type:"http"` + `--strict-mcp-config`). The agent is limited with `--tools "WebSearch,WebFetch"`, `--allowedTools "mcp__designer__*"` and `--permission-mode dontAsk`.
- **Risk:** `--bare` (the fast mode for scripts) needs an API key, which conflicts with R9. The docs say it may become the default for `-p` later.
- **Trusted domains:** WebFetch can be limited per domain, but WebSearch can't. Enforcing the list needs a hook in our Go server (untested).
- **Knowledge Base first** is up to the system prompt, which is fixed for the life of a conversation, even across `--resume`.
- **px0** starts a new process per turn (`--session-id`, then `--resume`). It uses no MCP, no streaming input and no tool limits.
- **Go MCP SDK** v1.8.0 is mature: Streamable HTTP, SSE, stdio and in-memory transports.
- **Not documented:** the stdin message format and the interrupt message (folded into the token spike).
- **Decisions raised:** moved to the new ticket "How is the Claude Code session configured?"
