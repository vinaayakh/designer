# Contributing to Designer

Thank you for helping improve Designer.

## How Designer is built

Designer is designed and written with AI coding agents, guided by the decisions recorded on its [planning maps](wayfinder/map.md) and the principles in [AGENT.md](AGENT.md): nothing changes a diagram without the user's approval, an append-only audit trail, plain-file Projects, token efficiency, and a single binary.

To keep the design coherent, **we don't accept outside pull requests**. Pull requests opened from forks will be closed with a pointer to this page. The most valuable contributions are clear bug reports and well-argued ideas, and they do shape what gets built.

## Report a bug

1. Search [existing issues](https://github.com/vinaayakh/designer/issues) first.
2. Open a [bug report](https://github.com/vinaayakh/designer/issues/new?template=bug_report.md) with:
   - your Designer version (`designer --version`), OS, browser and Claude Code version (`claude --version`);
   - what you did, what you expected, and what happened;
   - relevant lines from `<project>/.designer/logs/designer.log` (run with `--verbose` for more detail) and the browser console.

Check logs and screenshots before posting: they can contain your diagram's labels and your Project's path.

## Suggest a feature

Open a [feature request](https://github.com/vinaayakh/designer/issues/new?template=feature_request.md). Say what you're trying to do, what gets in the way today, and why it matters. If it touches a decision already on the planning maps, link the ticket and say what you'd change.

## Knowledge Base corrections

Records in `kb/records` describe compliance controls and design practices. If one is wrong, out of date or missing a source, open an issue with the Record ID and a link to the primary source. Please don't paste text from standards or vendor documentation into issues: many of those sources can't be copied ([ADR 0002](docs/adr/0002-knowledge-base-is-written-clean-not-share-alike.md)).

## Security issues

Don't open a public issue. See [SECURITY.md](SECURITY.md).

## What happens next

Maintainers triage each issue against the design principles and the roadmap. Accepted ones become tickets on the planning map or tasks for an agent; the issue links to the change when it lands.
