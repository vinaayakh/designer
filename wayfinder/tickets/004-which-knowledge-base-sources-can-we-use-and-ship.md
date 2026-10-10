---
id: "004"
title: "Which Knowledge Base sources can we use and ship?"
labels: [wayfinder:research]
status: open
assignee: claude (research subagent)
blocked_by: []
parent: map
---

# Which Knowledge Base sources can we use and ship?

## Question

Which authoritative sources should the Knowledge Base be built from, and which are we legally allowed to quote or summarise inside a shipped app?

Cover:
- Design practices: AWS Well-Architected Framework, Azure Well-Architected Framework, classic distributed systems references (CAP/PACELC, caching, queueing, replication patterns).
- Security: OWASP (ASVS, Top 10, Cheat Sheets), NIST CSF 2.0, NIST SP 800-53, CIS Controls / CIS Benchmarks, cloud security reference architectures.
- Compliance: SOC 2 Trust Services Criteria (**AICPA copyright: can we reproduce or must we paraphrase?**), the HIPAA Security Rule (45 CFR Part 164 Subpart C, public domain), HHS guidance, AWS/Azure HIPAA-eligible service lists and BAA terms.
- For each source: licence/terms, what we can ship (verbatim / paraphrase / link only), how often it changes, and a stable URL.
- A proposed **Trusted Source domain allowlist** for the web-search fallback.
- Which control lists can be checked by looking at a diagram, and which are process or policy only (feeds the Gap Report's "outside what the diagram can show" category).

## Context

Findings: branch `research/004-which-knowledge-base-sources-can-we-use-and-ship`, file `research/004-which-knowledge-base-sources-can-we-use-and-ship.md`.
