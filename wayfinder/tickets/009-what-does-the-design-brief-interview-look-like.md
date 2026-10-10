---
id: "009"
title: "What does the Design Brief interview look like?"
labels: [wayfinder:prototype]
status: closed
assignee: vinu (with claude)
blocked_by: []
parent: map
---

# What does the Design Brief interview look like?

## Question

What does the new-project interview look like and how does it flow? Required: target cloud (Generic/AWS/Azure), compliance regimes (none/SOC 2/HIPAA), CAP trade-off. Optional (default "unknown"): user count, performance (latency/throughput), read/write bias, availability target, data volume and growth, RPO/RTO, regions/data residency, budget sensitivity, team/ops maturity. Explain CAP to non-experts? A form or a conversation with the Assistant? Build a rough prototype to react to.

## Resolution

**Variant B, a wizard, with the Design Brief always visible beside it** (taken from Variant A). Prototype: branch `prototype/009-design-brief-interview`, file `prototypes/009-design-brief-interview/index.html` (open with `?variant=B`).

- **A form, not a conversation.** The interview runs locally in the UI and costs no tokens. The Assistant isn't involved. A chat interview (Variant C) and archetype presets (Variant D) were rejected.
- **Flow:** one question per screen, with a progress bar. The 3 required questions come first: target cloud, compliance regimes, CAP. Then a stop point: **Start sketching** or **Answer more**. Each of the 11 optional questions has a "Don't know — skip" button, plus a "Skip the rest" link. Skipped questions stay `unknown`, and the AI states its assumption for each.
- **The brief is always visible:** a panel next to the wizard shows the brief exactly as the AI will read it, updating with every answer. It shows `unknown` for unanswered questions and how many required answers are still missing.
- **The brief is required before sketching:** the Rough Draft opens only once the 3 required answers are in. The optional questions can be answered later from Project settings.
- **CAP is asked as a scenario**, not as theory: "The network between two of your servers breaks for two minutes. A user tries to save something. What should happen?" There are two answers: *make them wait or show an error* (CP, e.g. a bank transfer) or *accept it and sort out conflicts later* (AP, e.g. a social feed). An "I'm not sure — explain" link opens the short explanation. **CA is not offered**, because it only exists on a single machine.
- **Compliance** allows several choices: None, SOC 2, HIPAA. None can't be combined with the others. Answering "None" is different from not answering.
- **Optional questions and their answers**, as in the prototype: users, latency, peak throughput, read/write bias, availability, data volume and growth, RPO, RTO (asked separately), regions and residency, budget sensitivity, team/ops maturity. The exact answer ranges can be adjusted during the build.
