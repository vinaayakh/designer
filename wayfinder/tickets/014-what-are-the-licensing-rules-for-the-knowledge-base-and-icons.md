---
id: "014"
title: "What are the licensing rules for the Knowledge Base and icons?"
labels: [wayfinder:grilling]
status: closed
assignee: vinu (with claude)
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

## Resolution

Decided with vinu on 2026-10-10. This is our reading of the licence terms, not legal advice. Why we chose this: [ADR 0002](../../docs/adr/0002-knowledge-base-is-written-clean-not-share-alike.md).

1. **Commercial?** Designer is **free and open source, but will be used at work**, so for licensing it's treated as commercial. NonCommercial content (CIS, Microsoft Learn under its default terms) is never copied.
2. **Knowledge Base licence: written in-house, no share-alike.** The Knowledge Base is curated during development, shipped in the binary, and carries the app's own licence. How a Record may use each source tier:

   | Tier | Sources | How a Record may use them |
   |---|---|---|
   | Public domain | HIPAA Security Rule (45 CFR 164 Subpart C), NIST CSF 2.0, SP 800-53r5, SP 800-66r2 | Verbatim, with credit |
   | CC BY 4.0 | Azure Architecture Center | Adapted, with attribution |
   | CC BY-SA 4.0 | OWASP (ASVS, Top 10, Cheat Sheets), AWS docs incl. Well-Architected and the AWS Security Reference Architecture | **Facts and IDs only**, in our own words, plus a link. Never adapted from their prose |
   | Restricted | SOC 2 TSC, Azure Well-Architected, AWS compliance pages, HIPAA-eligible service lists | Facts and IDs only, in our own words, plus a link (service lists with an as-of date) |

   Rejected: shipping the Knowledge Base as a CC BY-SA data file, and **generating the Knowledge Base on first install** with the user's Claude Code. Generating would cost the user millions of tokens, give every install a different, unreviewed Knowledge Base (breaking R6's "curated"), and fail on sites that block bots.
3. **SOC 2:** we write our own control-intent descriptions keyed to criterion IDs (`CC6.1`, `A1.2`…) with a link to AICPA. No licence request in v1.
4. **CIS:** **out of v1 entirely**. No Records, no control numbers, not on the allowlist.
5. **Legal review:** not a v1 release gate. It becomes one if Designer is ever sold or taken over by a company. In its place, every source in a Record carries a `licence_tier`, and a **CI check** fails any Record whose CC BY-SA or Restricted source shares a long verbatim run (start at 12+ words) with that source text. The CI check needs a local copy of each source text to compare against.
6. **Trusted-domain allowlist (fetch):** `docs.aws.amazon.com`, `learn.microsoft.com` (used to answer questions only, never copied into Records), `owasp.org`, `top10.owasp.org`, `cheatsheetseries.owasp.org`, `nist.gov`, `csrc.nist.gov`, `govinfo.gov`, `ecfr.gov`, `federalregister.gov`, `hhs.gov`, and `aws.amazon.com/compliance/` (path-restricted; facts only).
   - Not fetched: `aicpa-cima.com` (link-only, its URLs live inside Records) and `doi.org` (a redirector). `cisecurity.org` is dropped entirely along with CIS.
   - Not added in v1: `azure.microsoft.com` and `cloudsecurityalliance.org`.
   - **Blocked sites:** HIPAA relies on the built-in Knowledge Base copy of 45 CFR 164 Subpart C, which is shipped verbatim. Live lookups prefer `govinfo.gov`. The system prompt says: if a fetch fails, cite the Knowledge Base Record instead.
7. **AWS/Azure icons: download on first use.** When the user first picks an AWS or Azure target, the app shows the vendor's icon terms. Once the user agrees, it fetches the official icon zip from the vendor's URL into a local cache, with a pinned version and checksum. Icons are never bundled in the binary.
   - **Fallback** (offline, or the user declines): nodes use generic Component Kind icons with the full service name as a label.
   - **Editor rule for all official icons:** rotation, flip, cropping and aspect-ratio changes are locked, and the full service name is always shown beside the icon (Azure's terms; applied to AWS too, for simplicity).
   - The icon-sync script also keeps the download manifest up to date (URL, version, checksum), because the vendor zip URLs change.

**Effects on other tickets:**
- "What is the Knowledge Base record format?" gets `sources[].licence_tier` and the verbatim-overlap CI check.
- "Write the 5 pilot Knowledge Base records" tests whether "facts and IDs in our own words" reads well.
- "What are the Component Kinds and Cloud Catalog contents?" gets the icon reference pointing at the downloaded cache, plus a generic fallback icon per Component Kind.
