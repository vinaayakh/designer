---
id: "014"
title: "What are the licensing rules for the Knowledge Base and icons?"
labels: [wayfinder:grilling]
status: open
assignee:
blocked_by: []
parent: map
---

# What are the licensing rules for the Knowledge Base and icons?

## Question

Using the findings from "Which Knowledge Base sources can we use and ship?", decide:
1. **Is Designer commercial?** That is, sold, or used inside commercial work. This decides whether NonCommercial sources (CIS, Microsoft Learn) can be used at all.
2. **Is CC BY-SA acceptable for the Knowledge Base data file?** Share-alike carries over from OWASP and AWS docs. Does the Knowledge Base ship as a separately licensed data file?
3. **SOC 2:** ask AICPA for a licence, or write our own descriptions keyed to criterion IDs?
4. **CIS:** in v1 as control numbers and links only, or left out?
5. **Legal review:** is it required before release?
6. **Trusted-domain allowlist:** accept the 11 proposed domains? And how do we handle sites that block automated fetches (hhs.gov, ecfr.gov), e.g. by relying on the built-in Knowledge Base copy?
7. **AWS/Azure icons** (from "How do we draw the Production Diagram?"): bundle them in the binary, have the app download them on first use after the user agrees to the vendor terms (the research recommends this), or ask AWS and Microsoft for written permission? Azure's terms also require locking icons against cropping, flipping, rotating or distorting, and showing the full service name next to them.
