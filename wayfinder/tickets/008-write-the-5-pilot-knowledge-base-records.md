---
id: "008"
title: "Write the 5 pilot Knowledge Base records"
labels: [wayfinder:task]
status: closed
assignee: vinu (with claude)
blocked_by: ["007"]
parent: map
---

# Write the 5 pilot Knowledge Base records

## Question

Using the format decided in "What is the Knowledge Base record format?", research and write about 5 pilot Records covering each kind (e.g. one design practice, one security practice, one SOC 2 control, one HIPAA Security Rule safeguard, one AWS/Azure-specific practice), each with proper source citations. Write down any problems with the format that writing them exposes.

Per "What are the licensing rules for the Knowledge Base and icons?", at least one pilot Record should rest on a CC BY-SA source (OWASP or AWS) written as facts and IDs in our own words. That tests whether a `facts-only` Record still reads well, and whether the CI overlap check passes.

Format decided in "What is the Knowledge Base record format?". Suggested pilot mix:
- One SOC 2 control (`soc2.cc6-1`) and one HIPAA control (`hipaa.164-312-a-2-iv`).
- One design practice and one security practice. At least one practice should be linked by `satisfied_by` and rest on a CC BY-SA source written as `facts-only`.
- One AWS/Azure-specific practice (restricted by `clouds`).

Also write `kb/eval/queries.yaml` (~30 queries with expected Record IDs) and report the top-5 hit rate from a throwaway BM25 run, along with any problems the format exposes.

## Resolution

Done on 2026-10-10 (AFK). **The format holds.** Five Records were written and pass every check. Writing them exposed eight problems: one fixed in the Records, the rest listed below for the build or later tickets.

**What was written** (on `master`)
- **Records:**
  - [`kb/records/soc2/cc6-1.md`](../../kb/records/soc2/cc6-1.md): SOC 2 control, our own words, `coverage: partial`.
  - [`kb/records/hipaa/164-312-a-2-iv.md`](../../kb/records/hipaa/164-312-a-2-iv.md): HIPAA control, rule text verbatim (public domain), addressable, with a note on the status of the rule change.
  - [`kb/records/practice/encryption-at-rest.md`](../../kb/records/practice/encryption-at-rest.md): security practice. **Facts-only** (OWASP Cryptographic Storage Cheat Sheet, CC BY-SA) plus NIST SP 800-111. Both controls link to it through `satisfied_by`.
  - [`kb/records/practice/cache-aside.md`](../../kb/records/practice/cache-aside.md): design practice. **Attribution** tier (Azure Architecture Center, CC BY 4.0, pinned to commit `0ab15d0`).
  - [`kb/records/practice/aws-s3-gateway-endpoint.md`](../../kb/records/practice/aws-s3-gateway-endpoint.md): AWS-only practice (`clouds: [aws]`). **Facts-only** (AWS docs, CC BY-SA).
- **Source copies:** [`kb/sources/`](../../kb/sources/README.md) holds the HIPAA §164.312 text, the OWASP cheat sheet, the AWS page text and the Azure pattern, each with its licence and retrieval date. `kb/.cache/` is gitignored for restricted sources.
- **Test set:** [`kb/eval/queries.yaml`](../../kb/eval/queries.yaml) has 32 queries, phrased the way the Assistant would ask.
- **Checker (throwaway):** branch `spike/008-pilot-knowledge-base-records`, folder `spikes/008-kbcheck/` (Go). It lints Records, runs the copy check and the BM25 test.

**Results**
- **Copy check:** passes. The longest verbatim run is 5 words against AWS and 3 against OWASP, so facts-only Records still read naturally. Negative test: pasting one 16-word OWASP sentence into a Record fails the check, so the 12-word threshold works. SOC 2 was skipped because there's no local AICPA copy, as designed.
- **Search** (hand-written BM25, fields weighted title > aliases > summary > body):
  - **Top-5 hit rate 100%**, but that means nothing with only 5 Records.
  - **Top-1 97%, MRR 0.98.**
  - The one miss ("network segmentation private subnet for databases") ranks the AWS endpoint practice first.
  - A naive stemmer made results *worse* (top-1 94%), which supports using aliases instead of stemming.
  - Until the Knowledge Base has many more Records, report top-1 and MRR alongside top-5.
- **Body size:** controls come to about 315 tokens. All three practices (four sections each) come to about 365–377 tokens, just over the 350 soft cap. Estimated as characters ÷ 4.

**Problems the format exposed**
1. **`sources[].copy` field added.** The CI copy check needs to know which reference file belongs to each source, so each source now has an optional `copy: kb/sources/...`. A facts-only source without one is skipped with a notice (restricted sources).
2. **Index the regime name and `ref` in the title field.** "SOC 2 access control requirements" ranked the HIPAA Record first until "SOC 2" / "HIPAA" were indexed with the title. The real `search_kb` must index `regime` (display name), `ref` and `id`.
3. **Key PHI Records on Data Classification, not Kind** (fixed in the pilot). `kinds: ["data.*"]` would miss PHI held in a queue or a search index. The HIPAA and encryption Records now match on `data_classes` only. "What are the Component Kinds and Cloud Catalog contents?" must allow a Data Classification on any Node that holds data, not only on `data.*` Kinds.
4. **Dangling `satisfied_by` links.** CC6.1 really needs practices such as an identity provider, least privilege and network segmentation, none of which exist yet. The pilot links only what exists. Recommendation: CI *warns* on links that don't resolve while the Knowledge Base is being written, and the release build *fails* on them. Write practices before the controls that cite them.
5. **`applies_to` can't express structure.** The S3 endpoint practice really applies to "compute in a private VPC talking to S3", and cache-aside to "a read path to a store". Matching by Kind over-applies (every S3 Node on AWS). That's fine because the AI judges, but Conversion should treat matched practices as candidates, not findings.
6. **The `brief` filter can hide a practice.** Cache-aside is filtered to `read_write_bias: read-heavy`. A "balanced" Brief hides it even when one hot path needs it. Recommendation: `brief` stays a hard filter for controls, but is a *ranking hint* for practices.
7. **The soft cap is tight for practices.** The four-section practices sit about 5–8% over it. Recommendation: 350 tokens for controls, 400 for practices. It's still only a warning.
8. **Some IDs are provisional.** Kind IDs (`data.relational`, `data.document`, `data.keyvalue`, `data.cache`, `data.object`) wait on "What are the Component Kinds and Cloud Catalog contents?", which must also include the Data Class `confidential`. Brief field IDs and answer values (`compliance: hipaa`, `read_write_bias: read-heavy`) aren't fixed anywhere, so a new ticket was added: "What are the Design Brief field IDs and answer values?"

**Source facts later tickets depend on**
- **HIPAA:** govinfo's annual CFR XML is fetchable, unlike hhs.gov and ecfr.gov. The §164.312 text is identical in the 2023 and 2025 editions. The proposed Security Rule overhaul (Dec 2024 NPRM, which would make encryption *required*) is not final. Per reginfo.gov it was moved to long-term actions with a target of July 2027 ([Alston & Bird, 2026-07](https://www.alston.com/en/insights/publications/2026/07/hhs-delays-hipaa-security-rule-2027), [Paubox](https://www.paubox.com/blog/hhs-pushes-final-hipaa-security-rule-update-to-2027)). HIPAA Records carry a "Rule status" note.
- **AWS docs:** the public `awsdocs` repos are archived, so AWS copies are taken from the HTML page. The source-refresh tooling needs a fetcher for each source.
