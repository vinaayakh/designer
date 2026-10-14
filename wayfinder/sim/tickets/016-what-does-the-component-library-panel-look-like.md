---
id: "016"
title: "What does the Component Library panel look like?"
labels: [wayfinder:grilling, wayfinder:prototype]
status: closed
assignee: claude (recommended options, for vinu's review)
blocked_by: ["013", "015"]
parent: map
---

# What does the Component Library panel look like?

## Question

Design the Component Library panel: how it's searched and browsed, how Components get onto the diagram, and where it appears (normal editing of the Production Diagram, and Sim Mode).

## Resolution

Decided 2026-10-15. Prototype: `spikes/sim-002-library/` (screenshots and search timings in `results/`, notes in its [RESULTS.md](../../../spikes/sim-002-library/RESULTS.md)).

**Where it appears.** One panel, docked on the left of the canvas, toggled with **C**:
- **Production Diagram editing:** placing a Component adds it to the diagram. It replaces the Inspector's "Add" with the Kind and Service dropdowns; the Inspector keeps editing the selected Node.
- **Sim Mode:** placing a Component adds it to the Scenario (ticket 006). Quick inserts replace the palette from ticket 008.
- **Rough Draft:** hidden. The draft stays a freehand napkin sketch.

**Layout, top to bottom:**
1. **Header:** "Components" and the target cloud. The cloud is fixed by the Brief; the switch exists only in the prototype.
2. **Search box:** focus with `/`, arrows move, Enter places, Esc clears.
3. **Quick inserts:** chips you drag onto an Edge or Node (ticket 013).
4. **Groups** in library order, collapsible, with their category icon and a count. Load balancing, Compute, ECS and EKS start open, the rest closed. Open and closed state is remembered per user.
5. **Footer:** the icon set credit ("AWS Architecture Icons 07312026, downloaded under AWS's terms"), or the reason for generic icons.

**A row** shows:
- the official icon (28 px);
- the label, with the full official name, or the Kind gloss, under it;
- a badge: Simulated, Pass-through or Diagram only for Nodes; Capacity pool or Group for Groups;
- "not HIPAA" when the Brief includes HIPAA and the Service isn't eligible (from the catalog).

Hovering shows the Service ID and the default Sizing.

**Search:**
- **Fields:** label and official name (weight 3); keywords, catalog aliases and Service ID (2); group title (1.5); Kind gloss (1).
- **Scoring:** a whole-word match scores 10, a prefix 6, a substring 3. Every query word must match somewhere.
- **Ties** go to simulated Components, then Nodes before Groups, then library order. "More services" entries are down-weighted.
- **Results stay grouped:** groups appear in order of their best match. Searching "ecs" shows the ECS group; "k8s" shows EKS; "redis" shows Caching.
- **Speed:** 0.7–1.4 ms per keystroke over the 103 AWS entries, so there's no index or debounce.

**Placing:**
- **Drag onto the canvas:** the Component lands where it's dropped. Dropping it inside a Group sets its parent (v1 ticket 006).
- **Drag onto an Edge or Node:** applies the matching quick-insert rule. For example, a cache dropped on an Edge into a database becomes cache-aside.
- **Click or Enter:** places beside the selected Node, or in the visible centre when nothing is selected, using the local placement used for Proposals (no ELK).
- **Cluster Components** (ECS and EKS clusters) draw a Group with a capacity pool (ticket 014). Network Components draw region, VPC, subnet and account Groups.
- **New Nodes arrive with the Component's default Sizing and Edge behaviour,** so they simulate immediately.

**Accessibility:** a keyboard path for everything (search, arrows, Enter), labelled rows, and badges that are text, not colour alone.
