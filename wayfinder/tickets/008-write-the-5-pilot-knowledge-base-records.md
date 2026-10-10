---
id: "008"
title: "Write the 5 pilot Knowledge Base records"
labels: [wayfinder:task]
status: open
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
