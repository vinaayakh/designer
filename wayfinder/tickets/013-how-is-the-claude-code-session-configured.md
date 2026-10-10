---
id: "013"
title: "How is the Claude Code session configured?"
labels: [wayfinder:grilling]
status: closed
assignee: vinu (with claude)
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

## Resolution

Approved by the user, 2026-10-10. CLI facts were checked by experiment on Claude Code 2.1.296 with a subscription login (about $0.04, Haiku). Raw outputs are in the session scratchpad, not committed.

1. **System prompt:** one **static** prompt shipped in the binary, the same for every Project. It covers role, Knowledge Base first, changes only through `submit_proposal`, respecting the brief (stating `unknown` fields as assumptions), how to read the compact format, and the trusted-source rule. Everything Project-specific goes in user messages: the brief, target cloud and full diagram in the first one, then change lists (brief edits included). Task instructions (Conversion, chat, Gap Report) go in the message that starts the task. Final wording is written when building.
2. **Trusted domains, two layers:**
   - **WebFetch** is limited by `--allowedTools "WebFetch(domain:…)"` rules built from the trusted list. These fail closed.
   - **Every** WebSearch/WebFetch call also goes through an **HTTP PreToolUse hook** to the Go server (`--settings`).
     - **WebSearch** is denied unless `allowed_domains` is set and is a subset of the trusted list (subdomains count). The model sees the reason and retries.
     - **WebFetch** to hosts off the list is denied, including Claude Code's built-in documentation sites that the permission rules let through.
   - HTTP hooks fail open, which is accepted: the hook shares a server with the MCP tools. The spike's custom `web_fetch` MCP tool is dropped.
   - The trusted list is curated, shipped in the binary and read-only. Its contents are decided in "What are the licensing rules for the Knowledge Base and icons?".
3. **Lifecycle:**
   - **Start:** the process starts when the Assistant sidebar first opens or the user starts typing, to hide the 4 s startup.
   - **Stop:** after 15 min idle, or 30 s after the last browser tab disconnects.
   - **Restart:** it comes back with `--resume`, which keeps the cache warm within the hour.
   - **Crash:** resume once automatically and resend the in-flight turn if it has no result yet. If that fails, start a fresh session with full context and show a quiet notice. Waiting Proposals live in the runtime, so they survive.
   - **Reopening the app:** resume if the last turn was less than 1 hour ago, otherwise start fresh.
4. **MCP transport:** Streamable HTTP on the runtime's own server (`/mcp`), bound to 127.0.0.1. A random bearer token is generated per process and passed in `--mcp-config` headers. Requests with a browser `Origin` header are rejected.
5. **Model:** a per-install setting (Default / Opus / Sonnet / Haiku) passed with `--model`, defaulting to "Claude Code's default". The model can be changed mid-session with the `set_model` control message, so a different model per task needs no second process (a choice left to "How does Conversion work?").
6. **`--bare`:** accept the risk. `--bare` already fails with a subscription login ("Not logged in"), and plain `-p` works. Check `claude --version` against a tested range at startup, warn outside it, set a minimum supported version, and show a clear error if login fails.
7. **Isolation from the user's own Claude Code config:** run in an empty working directory owned by the runtime (OS cache folder, per Project), never the Project folder. Flags: `--setting-sources "" --strict-mcp-config --disable-slash-commands --settings <ours> --system-prompt <ours>` plus `CLAUDE_CODE_DISABLE_AUTO_MEMORY=1`. Without these, a `CLAUDE.md`, the user's plugins and 91 slash commands leaked in (about 200 extra tokens per turn).

**Launch shape:**
```
claude -p --input-format stream-json --output-format stream-json --verbose \
  --system-prompt <ours> --model <setting> \
  --tools "WebSearch,WebFetch" \
  --allowedTools "mcp__designer__*,WebSearch,WebFetch(domain:…)…" \
  --permission-mode dontAsk \
  --mcp-config <http + bearer> --strict-mcp-config \
  --setting-sources "" --disable-slash-commands --settings <hook config> \
  [--resume <session-id>]
```

**Corrections to earlier findings:**
- The spike's `--allowedTools "mcp__designer__*"` on its own **denies all web access** in `dontAsk` mode, so WebSearch/WebFetch must be listed.
- WebFetch domain rules accept `*.example.com` wildcards.
- A redirect to another host comes back to the model as a notice rather than being followed.
