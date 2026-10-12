# Rough Draft, Brief and Conversion

## The Rough Draft

Sketch freely with Excalidraw's tools. Designer reads meaning from the drawing:

- a labelled shape is a **Node** (a component: "API", "Orders DB", "Payments");
- an arrow between two shapes is an **Edge**; its label says what flows;
- a shape drawn around others is a **Group** (a VPC, a subnet, a trust zone); Groups can nest;
- free text is a **Note** the Assistant reads as context.

Labels matter more than looks. "DB" is fine; "Postgres (orders, PII)" is better.

## The Design Brief

Before Conversion, a short wizard asks for the target (AWS, Azure or generic), expected users and throughput, latency, availability, RPO and RTO, data volume, regions and residency, compliance (SOC 2, HIPAA), budget sensitivity and your team's operations maturity. Every question accepts **Unknown**: the Assistant then picks a sensible value and records it as an open assumption in the Register for you to confirm later.

You can edit the Brief later from Settings (except the target cloud once the Production Diagram exists).

## Conversion

**Convert** runs once per Project:

1. **Pre-check** (local, instant): flags overlapping shapes, unlabelled Nodes and loose arrows so you can fix them first.
2. **Design**: the Assistant types every Node (a Component Kind and, on AWS or Azure, a cloud service), adds what production needs (load balancers, subnets, backups, key management and so on) and writes a **Design Rationale**: why each component exists, which Knowledge Base practices it applies, the trade-offs, and every assumption. Confirmed requirements and constraints in the Register are part of its brief.
3. **Conversion Preview**: the laid-out result appears for review. **Accept**, **Reply** with what to change, or **Reject**.

Accepting creates Production version 1, freezes the Rough Draft (read-only, kept for reference), imports the Rationale's assumptions and decisions into the Register, and starts a Gap Check.
