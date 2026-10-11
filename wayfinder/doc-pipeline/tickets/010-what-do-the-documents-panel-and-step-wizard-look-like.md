---
id: "010"
title: "What do the Documents panel and Step wizard look like?"
labels: [wayfinder:prototype]
status: open
assignee:
blocked_by: ["001", "002", "003"]
parent: map
---

# What do the Documents panel and Step wizard look like?

## Question

How does a user move through the pipeline? Build a rough, throwaway prototype to react to:

- **Documents panel:** each Step with its status (not started, needs answers, ready to generate, current, stale), what it depends on, and its latest version.
- **Step wizard:** the questions a Step needs, with the Brief always visible (as in the Brief wizard), `unknown` allowed, and what each answer will become in the Register.
- **Register view:** entries by type with status and source; confirm or reject an assumption; see which documents cite an entry and which would go stale.
- **Generation:** progress while the Assistant writes, chat questions it raises, and the result with its cited entries.
