# Prototype sim-002: the Component Library

Tickets: [sim 012–016](../../wayfinder/sim/map.md). Throwaway; not committed.

```
npm install
node scripts/fetch-icons.mjs   # downloads the pinned AWS icon package, checks sha256, extracts the 85 listed icons
npx vite build && node scripts/shots.mjs   # screenshots and search timings into results/
```

## What it shows

- **Library panel:**
  - Provider switch (AWS, Azure draft) and a search box (`/` to focus, arrows, Enter to place).
  - Quick-insert chips, then collapsible groups in the library file's order: Traffic sources, Load balancing, Edge and APIs, Compute, ECS, EKS, Databases, Caching, Messaging and streaming, Storage, Network, Security and identity, Monitoring, More services.
  - Each row has the official icon, the label, the full service name, a sim badge (Simulated / Pass-through / Diagram only, or Capacity pool / Group) and "not HIPAA" where it applies.
- **Placing:** click (beside the last placed) or drag onto the canvas. Nodes become 200×72 cards with the icon as a separate image; cluster and network components become Groups with their icon.
- **Data:** read straight from `internal/catalog/library/{aws,azure}.yaml`, `internal/catalog/data/*.yaml` and the icon manifest `internal/catalog/icons/aws.yaml`. There's no AWS- or Azure-specific code: Azure renders from the same code with generic Tabler icons.

## Results

- **Icons:**
  - The pinned package (`07312026`, 13.99 MB) downloads from AWS's CDN and matches its sha256.
  - Extracting only the 85 listed icons gives 468 KB on disk; the largest icon is 9 KB.
  - Every key in the library resolves to a file in the package.
- **Search** over the 103 AWS entries (83 curated Components plus 20 more catalog Services) takes 0.7–1.4 ms per keystroke. Top results:

| Query | First result |
|---|---|
| ecs | ECS cluster (Fargate) |
| redis | Amazon ElastiCache (Redis OSS) |
| load balancer | Application Load Balancer |
| postgres | Amazon RDS for PostgreSQL |
| queue | Amazon SQS |
| k8s | EKS cluster (managed node group) |
| kafka | Amazon MSK |
| nat | NAT gateway |
| fargate service | ECS service on Fargate |

Results stay grouped under their group heading.

## Design problems the prototype found (fixed in the tickets)

1. **Truncated card names.** A long official name ("Amazon ElastiCache (Valkey)", "Amazon Aurora PostgreSQL-Compatible Edition") got cut off on a 200×72 card. The icon terms rule (v1 ticket 014) needs the full name beside an official icon, so cards wrap the service name onto two lines and grow to 84 px when needed.
2. **Raw Kind IDs.** Row subtitles showed `network.load-balancer`; they show the Kind's gloss instead (fixed here).
3. **Group badges.** Groups showed "Simulated"; they now say Capacity pool (clusters on EC2) or Group. A Fargate cluster has no pool and should say Group.
4. **Ranking ties.** "ecs" ranks the cluster Group above the service. On a tie, Nodes should rank above Groups, since services are placed more often.
