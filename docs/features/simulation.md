# Simulation

Sim Mode runs traffic through your Production Diagram so you can see roughly how the design behaves and where it breaks first. You change sizing and add a cache in a **Scenario**, compare a test run against a **baseline**, and bring the changes you like back into the diagram through the usual review.

The numbers are approximate by design. Use them to find the right shape and the order in which things break, not to promise an exact capacity.

## Open Sim Mode

Click **Simulate** in the top bar. You need a Production Diagram, and no Proposal waiting for review. Sim Mode opens over the editor and shows the latest version, read-only. Moving dots show traffic on each Edge, and each component gets a coloured outline and a utilisation bar: green below 60% busy, amber to 85%, red above.

The **Scenario** panel on the right shows:

- **Components, busiest first.** Click one to inspect and change it.
- **How your diagram was read.** Everything the import had to assume, for example:
  - a self-hosted service (such as Keycloak on Fargate) is simulated as a container;
  - a CDN or load balancer splits traffic evenly across its routes until you set the shares;
  - outbound calls through NAT aren't counted per request;
  - services off the request path (Secrets Manager, KMS, DNS, logging) are left out.
- **Changes in this Scenario,** each with **undo**.

Sizing comes from the diagram's attributes where it can: `tasks: 2-4` means 2 tasks scaling to 4, and `instance: db.t4g.medium` sets the size. Traffic and the latency target come from the Design Brief (throughput, latency, read/write bias). Everything else uses the component's Behaviour Profile.

## Baseline, change, test

1. **Run baseline.** This records how the diagram behaves as drawn: p50, p95 and p99 at your traffic, errors, cost per hour, and the **breaking point**, where traffic ramps until p99 misses the target or errors pass 1%. It also records which components were busiest at that point. A run takes a few seconds and doesn't disturb the live view.
2. **Change things.** Click a component to set its instances, size, autoscaling (on requests per instance), and how many calls it makes per request to each Node it calls. On a database or other data store, **Add a cache in front** puts a cache in front of it that answers 90% of reads. The live view restarts with each change.
3. **Run test** to see the same numbers beside the baseline.

The traffic slider and the 1× / 10× / 100× speed buttons affect only the live view. Runs always use the Scenario's traffic and the same random seed, so baseline and test compare fairly.

Costs leave out components without a price yet (shown as "excl. N").

## Apply to diagram

**Apply to diagram (N)** turns the Scenario's changes into a Proposal that you review like any other, marked "Your changes from Sim Mode":

- Sizing is written as readable attributes: `instances`, `autoscaling` (e.g. `2-10, target 60% requests`) and `instance`. These replace older free-text keys such as `tasks`.
- An added cache becomes a new Node, from your cloud's catalog (ElastiCache for Valkey on AWS), next to the store, with an Edge from its caller.
- A component you marked as self-hosted gets `simulate_as`, so the next import reads it the same way.
- The summary quotes the baseline and test numbers.

Step through and **Accept** or **Skip** each change. Skipped changes stay in the Scenario. There's no Reply: the Assistant didn't write these changes. Calls per request stay in the Scenario; they aren't drawn on the diagram.

After you accept, the diagram moves to a new version. Next time you open Sim Mode, click **Rebase on vN**. It drops the changes that are now in the diagram and keeps the rest, and your baseline is kept for comparison.

## Where it's stored

- `sim/scenarios/baseline.yaml` in the Project folder: the Scenario's base version, seed, traffic, changes and the last baseline and test results. It's plain YAML, so it diffs well.
- The Behaviour Profiles (the numbers behind each component) are built into Designer; see [internal/catalog/profiles](../../internal/catalog/profiles/README.md).

## Limits in this version

- One Scenario per Project (`baseline`).
- Changes you can make: sizing, autoscaling, calls per request, and a cache in front of a data store. The full Component Library, failure events (kill a Node, lose a zone), traffic shapes and the monitoring charts are specified but not built yet ([Simulation map](../../wayfinder/sim/map.md)).
- Prices: many are unverified or missing; see the Profiles README.
