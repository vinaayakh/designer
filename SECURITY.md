# Security

## Reporting a vulnerability

Please report vulnerabilities privately through GitHub: on the repository's **Security** tab, choose **Report a vulnerability** ([direct link](https://github.com/vinaayakh/designer/security/advisories/new)). Don't open a public issue.

Include the version (`designer --version`), your OS, steps to reproduce, and the impact you see. You'll get an acknowledgement within a week. Fixes are released as soon as they're ready, and the advisory credits you unless you'd rather not be named.

Only the latest release gets security fixes while Designer is in 0.x.

## Security model

Designer is a local, single-user tool. What it protects, and how:

- **The local server** listens on 127.0.0.1 only. It rejects requests whose Host isn't `127.0.0.1` or `localhost` on its own port (blocking DNS rebinding), requests from other origins, and state-changing requests without the `X-Designer` header, so other websites in your browser can't drive it.
- **The Assistant** is your own Claude Code, started by Designer with only Designer's MCP tools plus WebSearch and WebFetch. It has no file, shell or editing tools, ignores your Claude Code settings and other MCP servers, and every web call passes a hook that allows only a fixed list of trusted documentation domains. The hook fails closed.
- **The MCP and hook endpoints** require a bearer token generated at each start and passed only to the Claude Code process Designer launches.
- **Changes are reviewed:** the Assistant can only propose diagram changes and Register entries; nothing is applied until you accept it.
- **The Register** is hash-chained, so edits made outside Designer are detected and shown. This detects tampering; it doesn't prevent it.
- **Releases** ship with SHA-256 checksums and signed build provenance; the installers verify the checksum before installing.

Out of scope: anyone who can already run code as your user or read your Project folder, and Claude Code itself (report those to Anthropic). Designer sends your diagram, Brief and Register content to Claude through Claude Code; don't put secrets in them.
