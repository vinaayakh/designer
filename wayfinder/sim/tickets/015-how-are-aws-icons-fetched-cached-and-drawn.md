---
id: "015"
title: "How are AWS icons fetched, cached and drawn?"
labels: [wayfinder:grilling, wayfinder:research]
status: closed
assignee: claude (recommended options, for vinu's review)
blocked_by: ["012"]
parent: map
---

# How are AWS icons fetched, cached and drawn?

## Question

vinu: use icons from AWS. [v1 ticket 014](../../tickets/014-what-are-the-licensing-rules-for-the-knowledge-base-and-icons.md) already decided they're **never bundled**: downloaded on first use after the user agrees to AWS's terms, with a pinned version and checksum, falling back to generic icons, locked against edits, and always shown with the full service name. Nothing of that is built yet. Pin it down: which package, which files, where they're cached, how they get into diagrams, and how releases are followed.

## Resolution

Decided 2026-10-15. Verified against the real package: the prototype's `scripts/fetch-icons.mjs` downloads it, checks the checksum and extracts the files.

**The package** is AWS Architecture Icons, from the official icons page, released quarterly (end of January, April and July). The current release is `07312026`: 13.99 MB, 3,620 SVGs in four families:

| Family | Size used | Used for | Key example |
|---|---|---|---|
| Service icons | 64 px | most Components | `Arch_Amazon-Elastic-Container-Service_64` |
| Resource icons | 48 px | parts of a service: ALB, NLB, ECS Service and Task, ElastiCache engines, Aurora engines, the SQS queue, VPC gateways; also the Users, Client and Internet sources | `Res_Elastic-Load-Balancing_Application-Load-Balancer_48` |
| Group icons | 32 px | Region, VPC, subnets, account, Auto Scaling group | `Virtual-private-cloud-VPC_32` |
| Category icons | 64 px | library group headers | `Arch-Category_Compute_64` |

**Icon key = the file name without `.svg`.** That's unambiguous, and the sync script can diff it between releases.

**The manifest** is [internal/catalog/icons/aws.yaml](../../../internal/catalog/icons/aws.yaml): version, URL, sha256, byte size, the icons page and terms URLs, and `files` (key → path inside the zip) for exactly the keys the library and catalog use (85 today). It fills the catalog's existing `icon_set` (version, URL, sha256), and Services' `icon` fields take keys from the same set.

**Download and cache** (Go runtime, not the browser):
1. The **consent dialog** appears the first time an AWS target is chosen or the library panel opens on AWS. It links to the icons page and AWS's terms, and offers Download (about 14 MB, once) or Use generic icons.
2. The runtime downloads the zip and **checks the sha256**. On a mismatch it stops and keeps the generic icons.
3. It extracts only the manifest's files with `archive/zip` into the user cache: `os.UserCacheDir()/designer/icons/aws/07312026/<key>.svg` (468 KB for 85 icons). The zip is deleted afterwards.
4. It serves them at `/icons/aws/<key>.svg`.

The cache is per user, not per Project, and nothing is written into the binary or the repository.

**Into diagrams.**
- A Production Node's icon is a separate Excalidraw image element (v1 ticket 006). The SVG is embedded in the scene's `files` with the ID `aws-07312026-<key>`, so a `.excalidraw` file still opens on excalidraw.com and exports carry their icons. Each icon is a few KB, and identical icons share one file entry.
- Icon images are **locked** (no rotate, flip, crop or resize on their own), and the **full official service name is always shown** on the card. Cards wrap the service name onto two lines and grow from 72 to 84 px instead of truncating it (the prototype found truncation; this refines [v1 ticket 016](../../tickets/016-how-is-the-production-diagram-laid-out-and-styled.md)'s fixed 200×72).
- Declined, offline, or a key missing from the cache: the generic Kind icon (Tabler) plus the full name, as decided in v1 ticket 014.

**New releases.** The dev-time **icon-sync script**:
- downloads the latest package from the icons page;
- rewrites the manifest (version, URL, sha256, paths);
- reports new, removed and renamed keys against the library and catalog.

CI fails on a library or catalog key that's missing from the manifest. Existing diagrams keep their embedded icons until the user picks **Refresh icons**.

**Azure** follows the same steps with its own manifest (`icons/azure.yaml`) for the Azure Architecture Icons package. Azure's terms forbid modifying icons and require the name beside them, which the lock-and-name rule already covers.
