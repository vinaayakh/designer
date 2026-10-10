# 2. The Knowledge Base is written in-house and carries no share-alike licence

Date: 2026-10-10
Status: Accepted

Many of the best sources (OWASP, AWS docs) are CC BY-SA 4.0. Records adapted from them would force the whole Knowledge Base to be share-alike. SOC 2 (AICPA), CIS and Microsoft Learn can't be copied at all. Designer is free and open source but will be used at work, so NonCommercial content is off-limits. We decided that Records **copy verbatim only from public-domain sources** (HIPAA rule, NIST), **adapt only CC BY sources** (Azure Architecture Center) with attribution, and **use CC BY-SA and restricted sources as facts, IDs and links only, written in our own words**. Each source in a Record carries a `licence_tier`, and CI rejects long verbatim overlaps with non-free sources.

## Considered options

- **Ship the Knowledge Base as a separately licensed CC BY-SA data file.** Legally clean, but it puts a share-alike licence on content we want to control, and it still doesn't cover SOC 2 or CIS.
- **Generate the Knowledge Base on first install with the user's Claude Code.** Nothing is redistributed, but it costs each user millions of tokens, every install ends up with a different, unreviewed Knowledge Base (no longer "curated"), it needs the network and fails on sites that block bots, and AICPA objects to LLM use either way.

## Consequences

- Writing Records is slower: OWASP and AWS guidance has to be restated, not adapted. The pilot Records test whether this reads well.
- Close paraphrase of CC BY-SA text still counts as adaptation, so the CI overlap check is a safeguard, not a guarantee.
- Legal review isn't required for v1. It becomes required if Designer is ever sold.
