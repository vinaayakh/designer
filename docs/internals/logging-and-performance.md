# Logging and performance

How to see what Designer is doing, find slow spots, and measure canvas smoothness.

## Logs

Designer writes JSON-lines logs to `<project>/.designer/logs/designer.log` (rotated at 20 MB, 5 kept) and readable lines to stderr.

- `--verbose` (or `--log-level debug`): every request, MCP tool call, timer, browser timing and resource sample. The default `info` logs startups, Assistant turns and every warning or error.
- `--slow-ms 500`: requests, tool calls and tasks slower than this are logged as `slow request` / `slow tool call` / `slow task` warnings. Assistant turns are slow after 3 minutes.
- `--sample 15s`: how often CPU (% of one core) and memory (RSS) of Designer and Claude Code are sampled; sustained CPU over 90% or memory over 1 GB (Designer) / 2 GB (Claude Code) is a warning. `0` turns sampling off.
- `--log-dir <dir>`: write the log somewhere else.

Each Assistant turn logs `assistant turn` with its kind, model, wall time, time to first token, tokens in/out/thinking, cache reads/writes and cost. The browser sends its own timings to the same log (`"source":"web"`): API round trips, ELK layout, routing, rendering, review build/swap, export images, long tasks (> 200 ms is a warning), dropped frames while panning, JS heap, and uncaught errors. `GET /api/metrics` returns this run's totals: per-route and per-task counts with p50/p95/max, token totals per turn kind, and the latest and peak CPU/RAM samples.

To find problems: `grep '"level":"WARN"\|"level":"ERROR"' designer.log`, or filter by `"msg":"slow request"`, `"msg":"assistant turn"`, `"msg":"dropped frames"`, `"msg":"resources"`.

`npm run bench:pan -- [url] [--throttle 4] [--dpr 2] [--rough] [--hz 120] [--no-snapshot]` (in `web/`, needs Microsoft Edge) measures frame times, dropped frames, long tasks and script time while panning with the wheel, pinch-zooming, dragging with the hand tool and panning at 1x zoom. Saving is blocked, so it is safe on a real project. Build with `npx vite build --minify false` for readable profiles.

While you pan or zoom (wheel, trackpad, hand tool, Space + drag, middle button), the canvas moves a snapshot of the last frame instead of letting Excalidraw redraw every element, and hands the final position to Excalidraw when the gesture ends (`web/src/snapshotPan.ts`). To compare with Excalidraw's own handling, run `localStorage.setItem("designer.snapshotPan", "off")` in the browser console and reload.

For frontend work, run `bin/designer --no-open .` and `npm run dev` in `web/` (Vite proxies `/api` to port 4717).

## Command-line flags

| Flag | Default | What it does |
|---|---|---|
| `--port` | 4717 | preferred port on 127.0.0.1; a free one is used if it's taken |
| `--no-open` | off | don't open the browser |
| `--claude` | `claude` | the Claude Code command |
| `--log-level` | `info` | `debug`, `info`, `warn` or `error` |
| `--verbose` | off | same as `--log-level debug` |
| `--log-dir` | `<project>/.designer/logs` | where `designer.log` goes |
| `--slow-ms` | 500 | requests, tool calls and tasks slower than this are warnings |
| `--sample` | 15s | CPU and memory sampling interval (`0` turns it off) |
| `--version` | | print the version and exit |
