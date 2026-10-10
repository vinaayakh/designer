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

## Findings (awaiting user approval)

Harness and full numbers: branch `spike/012-how-few-tokens-can-the-assistant-use`, file `spikes/012-token-spike/RESULTS.md`.
- **Model:** most runs used Haiku (claude-haiku-5-5); two confirming runs used Opus 5.5 and pointed the same way.
- **Cost:** about $1.62 in list-price estimates.
- **Noise:** 2–3 repeats per condition, so differences under about 15% are noise.

| Condition | Result | Recommendation |
|---|---|---|
| (a) Raw JSON vs compact format | 50 nodes: 112k vs 2.3k tokens (49×). 300 nodes: 696k vs 14.5k. A 4-turn session cost 40× more with raw ($0.179 vs $0.0045). | **Compact format only** |
| (b) Inline vs fetched by tools | Fetching by tools added +13% input (Haiku), +33% input and +30% time (Opus), because the model fetched the whole diagram anyway. | **Put the brief (~68 tokens), selected nodes (~162) and the first diagram in the prompt** |
| (e) Change list vs full resend | Full resend added +33% input at 50 nodes and +62% at 300 (final context 67k vs 25k). A change list is about 109 tokens vs 3.1k. | **Only changes after the first turn** |
| (c) Long-running vs fresh process | Token use about the same: the 1-hour server-side cache carries across `--resume`, and 86% of input was cache reads. A fresh process adds about 3 s per turn. | **Long-running process** (for speed) |
| (d) About 6 vs about 20 tools | Twenty tools cost only +1.6k tokens of definitions, but the model used the extra ones: API calls went from 8.7 to 13, input +98%, +8 s. | **About 6 tools** |
| (f) KB catalog vs search only | About 41 tokens per record. About the same at 50 records (+4%). | **Search + `get_record`**; maybe a filtered catalog |
| System prompt | Replacing the default rather than appending saves about 3.2k tokens per call (−38% input). | **Replace** (input to "How is the Claude Code session configured?") |

**Main lesson:** tokens per turn ≈ context size × number of API calls. Cutting tool round trips matters as much as shrinking context.

**Confirmed CLI behaviour (Claude Code 2.1.296):**
- **Messages in:** stdin takes `{"type":"user","message":{"role":"user","content":"..."|[text blocks]}}`, one JSON object per line. Messages sent back to back are queued. **A malformed line kills the process.**
- **Cost reporting:** `total_cost_usd` is a running total for the session, and it carries across `--resume`.
- **Interrupting a turn:** send `{"type":"control_request","request_id":"x","request":{"subtype":"interrupt"}}`. It returns a `control_response` and an `error_during_execution` result, and the process stays usable. The interrupted turn's tokens are reported as 0. CTRL_BREAK on Windows kills the process.
- **Permissions:** `--permission-mode dontAsk` with `--allowedTools mcp__designer__*` gave 0 permission denials in 24 sessions.
