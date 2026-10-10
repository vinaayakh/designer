---
id: "004"
title: "Which Knowledge Base sources can we use and ship?"
labels: [wayfinder:research]
status: closed
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

## Resolution

Full findings: branch `research/004-which-knowledge-base-sources-can-we-use-and-ship`, file `research/004-which-knowledge-base-sources-can-we-use-and-ship.md`. This is not legal advice; the findings quote the licence terms behind each tier.

**Recommended: three licence tiers.**
1. **Public domain, can ship verbatim:** HIPAA Security Rule (45 CFR 164 Subpart C), NIST CSF 2.0, SP 800-53r5 (5.2.0), SP 800-66r2.
2. **Open licence, can ship with attribution:** OWASP ASVS 5.0 / Top 10 / Cheat Sheets (CC BY-SA 4.0); AWS docs including Well-Architected (CC BY-SA 4.0 per the AWS Site Terms); Azure Architecture Center (CC BY 4.0). **Share-alike** means Records adapted from these must also be CC BY-SA, so the Knowledge Base would ship as a separately licensed data file.
3. **Restricted, paraphrase or link only:**
   - SOC 2 TSC (AICPA, all rights reserved). AICPA's terms explicitly object to their content in LLM knowledge bases, so we write our own descriptions keyed to criterion IDs (e.g. CC6.1).
   - CIS Controls and Benchmarks (NonCommercial, Controls also NoDerivatives).
   - Azure Well-Architected (Microsoft Learn terms).
   - AWS/Azure HIPAA-eligible service lists: facts only, with an as-of date and a link. The AWS BAA is behind an NDA.

**Also:**
- Proposed trusted-domain allowlist: 11 domains. aicpa-cima.com is link-only (not fetched); cisecurity.org is links only.
- The HIPAA Security Rule rewrite (proposed Jan 2025) is still not final, so HIPAA Records need an as-of date.
- hhs.gov and ecfr.gov blocked automated fetches, which affects the web-search fallback.
- Controls are classified as diagram-checkable, partial, or process-only. This is the research agent's judgement, not taken from a source.

**Decisions raised:** moved to the new ticket "What are the Knowledge Base licensing rules?"
