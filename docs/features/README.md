# Feature guides

How to use Designer, in the order you'll meet each part.

1. [Getting started](getting-started.md): install, open a Project, the screen layout.
2. [Rough Draft, Brief and Conversion](conversion.md): sketch, answer the Design Brief, turn the sketch into a Production Diagram.
3. [Proposals and versions](proposals.md): ask for changes in chat, review them, move between versions.
4. [Gap Check](gap-check.md): check the design against SOC 2, HIPAA and security practices, and fix what's missing.
5. [Documents and the Register](documents.md): write the requirements spec, design doc, threat model and reliability plan; review the audit trail.
6. [Export](export.md): images, the arc42 document, and the document pack.
7. [Simulation](simulation.md): run traffic through the Production Diagram, find where it breaks, and apply what you changed.

## Limitations in this version

- **Icons:** every card uses a generic Tabler icon. Official AWS and Azure icons (downloaded on first use, after accepting their terms) are not built yet.
- **Knowledge Base:** only a few pilot Records. The Assistant falls back to trusted web sources for the rest.
- **Catalog facts:** the HIPAA eligibility flags in the AWS and Azure catalogs have not been verified against the vendors' lists.
- **Versions:** you can view and restore any version, but there is no side-by-side diff.
- **Chat history** is not saved; the Assistant's memory of a session is Claude Code's own.
- **Updating:** run the installer again; there is no `designer --update` yet.
