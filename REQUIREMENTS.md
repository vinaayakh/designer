# Requirements: AI-Assisted Architecture Design Tool

> Living record of what we're building toward. Status markers: ✅ confirmed · ❓ open question · 🔬 needs research.
> Last updated: 2026-10-10

## Vision

A diagramming tool for software architects that keeps the low-friction, hand-drawn feel of Excalidraw for **rough drafts**, then uses AI to turn a rough draft into a **production-ready architecture diagram** that follows established design, security, and compliance best practices — optionally targeted at a specific cloud provider.

## Core requirements (as stated)

### R1 — Rough-draft canvas
- ✅ Freeform, Excalidraw-like sketching surface for quick architecture mockups.
- ✅ Built on the **Excalidraw library** (embedded), with a layer that tags each shape with the component it represents. Needs a spike to confirm this works.
- ✅ **Must load fast and stay quick on large diagrams.** Package and bundle choices are judged on load time and rendering speed.
- ✅ **Measured:** about 0.4 s to usable, and smooth at 500–2,000 elements. Pinned `@excalidraw/excalidraw@0.18.0-143b5b6`.
- ✅ The frozen Rough Draft is read-only, but Nodes can still be clicked into the AI's context, using our own click handling.
- ✅ The Rough Draft should feel like a **back-of-the-napkin sketch**: fast and loose.
- ✅ Once the first Production Diagram is created, the Rough Draft is **frozen (read-only)**. It stays as a reference for both the user and the AI.

### R2 — AI conversion to a production-ready diagram
- ✅ AI converts the rough draft into a production-ready diagram using design best practices.
- ✅ "Production-ready" output for v1 =
  - (a) a clean, laid-out diagram using the target cloud's service icons,
  - (b) a **design rationale** (why each component exists, which practices it applies, trade-offs),
  - (c) a **compliance/security gap report** against the selected checklists (SOC 2 / HIPAA).
- ✅ Conversion produces Production version 1, and every accepted Proposal or edit saves a new version. Versions can be compared.
- ✅ The Production Diagram is **fully editable** and becomes the working source once it exists. Edits save new versions.
- ✅ **Conversion** ([How does Conversion work?](wayfinder/tickets/011-how-does-conversion-work.md)), run **once per Project**:
  - **Local pre-check** (no AI): flags partial overlaps, unlabeled Nodes and loose arrows.
  - **Design** (Conversion model setting, default Opus): types Nodes and writes a structured Design Rationale with Assumptions. It is reviewed all or nothing in a **Conversion Preview**, with Accept / Reply / Reject. Accept creates version 1 and freezes the Rough Draft.
  - **Gap Check**: runs on the accepted version, and can be run again on any version with **Check again**. The runtime puts the Records that apply in the message and files `process` Controls itself.
- ✅ **Layout and style** ([How is the Production Diagram laid out and styled?](wayfinder/tickets/016-how-is-the-production-diagram-laid-out-and-styled.md)):
  - **ELK** runs in a lazily loaded Web Worker, left to right, using Rough Draft positions as hints.
  - **Elbow arrows**, styled by interaction (sync solid, async dashed, stream dotted).
  - **Cards:** fixed 200×72 with icon, label, Service name and a Data Classification badge.
  - **Groups:** styled rectangles that grow automatically.
  - **Proposal changes** are placed locally inside their Group, without a full re-layout.
  - **Tidy up** re-runs layout on demand.
  - Generic icons are **Tabler**.
- ✅ **Design Rationale:** per-Node entries (why, practices, citations), plus Assumptions and trade-offs. It is kept current by accepted Proposals and saved as a snapshot per version.
- ✅ **Gap Report:** one item per Control that applies (met / unmet / outside, Nodes, finding, practice fixes), plus a **security recommendations** section. Saved as JSON with the version it checked and stamped with the Knowledge Base version. It shows a banner when it's older than the current version.
- ✅ **Citations** are Record IDs or trusted-list URLs, validated by the runtime when submitted.
- ✅ **Fixes** are drafted on demand (**Fix** / **Fix all**), so there is at most one Proposal waiting at a time.
- ❌ Infrastructure-as-code generation is not in v1.
- ❌ Re-converting from the frozen Rough Draft is not in v1.

### R3 — Cloud-provider targeting
- ✅ If a cloud provider is specified, generated components use that provider's services (e.g. AWS, Azure, GCP).
- ✅ v1 targets: **Generic** (provider-neutral), **AWS**, **Azure**.
- ✅ Provider services are kept as a data catalog, so adding providers (e.g. GCP) needs no code changes.
- ✅ **Catalog contents** ([What are the Component Kinds and Cloud Catalog contents?](wayfinder/tickets/017-what-are-the-component-kinds-and-cloud-catalog-contents.md)): `catalog/kinds.yaml` (48 Kinds) plus one YAML file per cloud with about 80–120 Services each. Each entry has an ID, the full official name, Kind, icon key, HIPAA-eligible flag, docs link and aliases. Multi-engine products get one entry per engine. Off-catalog services (e.g. third-party SaaS) are a Kind plus a name, shown with the generic icon. A dev-time icon-sync script diffs each vendor release, and CI checks Kind coverage, icon keys and the ≤ 1.2k-token export.
- ✅ **AWS/Azure icons are never bundled.** When the user first picks an AWS or Azure target, they agree to the vendor's icon terms, and the app downloads the official icon set into a local cache (pinned version, checksum). Offline or declined: generic Component Kind icons plus the full service name. Official icons are locked against rotate, flip, crop and distort, and always show the full service name.

### R4 — AI helper sidebar
- ✅ AI assistant lives in a sidebar, available in **both** the Rough Draft view and the Production Diagram view.
- ✅ The assistant has read access to the contents of the design view.
- ✅ Clicking a node on the canvas (in either view) adds it to the context of the message being sent to the assistant.
- ✅ Proposals apply to whichever view is active. Gap Report fixes apply to the latest Production Diagram.
- ✅ The assistant can **propose** changes to the Rough Draft. The user reviews each proposal and can **accept**, **reject**, or **reply with changes** (counter-suggest), and the assistant then revises it. Nothing is applied without the user's approval.
- ✅ **Reviewing a Proposal:** changes are drawn as an overlay on the canvas (added / removed / changed / moved). The user steps through them one at a time (accept or skip), or jumps to a full review of everything not skipped with a single Accept. **Parts of a Proposal can be accepted on their own**, and the runtime handles dependencies. Accepted changes are saved as **one new version with one Undo**. **The canvas is locked while a Proposal is waiting for review.** Payload and details: [How are Proposals shown and reviewed?](wayfinder/tickets/010-how-are-proposals-shown-and-reviewed.md).

### R5 — Project setup interview
When starting a new project, the user is asked for the system's non-functional targets, including:
- ✅ CAP theorem trade-off (which two of Consistency / Availability / Partition tolerance are prioritized).
- ✅ Expected number of users.
- ✅ Performance expectations (latency, throughput).
- ✅ Read-optimized vs write-optimized workload.
- ✅ Also: availability target, data volume and growth, RPO/RTO, regions and data residency, compliance regimes (none / SOC 2 / HIPAA), target cloud, budget sensitivity, team/ops maturity.
- ✅ Required: target cloud, compliance regimes, CAP. Everything else is optional and defaults to "unknown", which the AI states as an assumption.
- ✅ **Interview shape:** a local wizard (no AI, no tokens), one question per screen, with the brief always visible beside it. The 3 required questions come first and must be answered before the Rough Draft opens. Then the user can start sketching or answer more, and optional questions can be skipped or answered later from Project settings.
- ✅ CAP is asked as a plain-language network-split scenario, with an optional explanation. Only **CP** and **AP** are offered (CA isn't possible on more than one machine).
- ✅ **Fields and storage** ([What are the Design Brief field IDs and answer values?](wayfinder/tickets/018-what-are-the-design-brief-field-ids-and-answer-values.md)): 15 fields with fixed values plus `unknown` (regions and residency are separate), and an optional `notes` field. Saved as `brief.yaml` with every field written out, and validated against a brief schema built into the binary. Knowledge Base Records filter on value lists: a hard filter for Controls, a ranking hint for Practices.

### R6 — Internal knowledge base
An internal, curated database of records that the AI consults **first**:
- ✅ Software design best practices.
- ✅ Security best practices.
- ✅ Compliance checklists: **SOC 2** and **HIPAA**.
- ✅ Records are produced via deep research as part of the app development process.
- 🔬 Deep research needed to author these records from authoritative sources.
- ✅ v1: the Knowledge Base is **shipped with the app, curated, and read-only**. Users can't add their own records in v1.
- ✅ **Licensing** ([ADR 0002](docs/adr/0002-knowledge-base-is-written-clean-not-share-alike.md)): the Knowledge Base is written in-house during development and carries the app's own licence, with no share-alike. How a Record may use each source tier:
  - **Public domain** (HIPAA rule, NIST): verbatim, with credit.
  - **CC BY** (Azure Architecture Center): adapted, with attribution.
  - **CC BY-SA** (OWASP, AWS docs) and **restricted** sources (SOC 2, Azure Well-Architected, AWS compliance pages): facts and IDs only, in our own words, plus a link.
  - SOC 2 is described in our own words, keyed to criterion IDs. **CIS is out of v1.**
  - Each source in a Record carries a licence tier, and CI rejects copied text from non-free sources. No legal review until Designer is sold.
  - Generating the Knowledge Base on first install was rejected (token cost, not curated).
- ✅ **Record format** ([What is the Knowledge Base record format?](wayfinder/tickets/007-what-is-the-knowledge-base-record-format.md)):
  - One markdown file per Record, of two types: **controls** (one SOC 2 criterion or HIPAA implementation specification each) and **practices** (design or security).
  - Each Record says which Nodes and Brief answers it applies to, how far a diagram can show it, which practices satisfy it (controls only), and its sources with licence tiers and as-of dates.
  - Compliance is judged by the AI from the Records that apply to the diagram, not by rules in the runtime.
  - Search is a hand-written BM25 that returns IDs and short summaries. The Knowledge Base has a date version, stamped on every Gap Report.
- ✅ **Pilot done** ([Write the 5 pilot Knowledge Base records](wayfinder/tickets/008-write-the-5-pilot-knowledge-base-records.md)): five Records in `kb/records/`, source copies in `kb/sources/`, and a 32-query test set in `kb/eval/queries.yaml`. Facts-only Records read well and pass the copy check. Search puts the right Record first 97% of the time. Each source can name its local `copy` for the check.

### R7 — Trusted web search fallback
- ✅ When the knowledge base lacks needed information, the AI may search the web.
- ✅ Trusted Sources are a **curated domain allowlist shipped in the binary**, read-only in v1. It is **enforced, not just requested**: WebSearch must name allowed domains, and WebFetch only reaches listed domains. Details: [How is the Claude Code session configured?](wayfinder/tickets/013-how-is-the-claude-code-session-configured.md).
- ✅ **The list** (fetch allowed): `docs.aws.amazon.com`, `learn.microsoft.com` (used to answer questions only, never copied into Records), `owasp.org`, `top10.owasp.org`, `cheatsheetseries.owasp.org`, `nist.gov`, `csrc.nist.gov`, `govinfo.gov`, `ecfr.gov`, `federalregister.gov`, `hhs.gov`, and `aws.amazon.com/compliance/` (path-restricted). AICPA is link-only and never fetched. hhs.gov and ecfr.gov block bots, so HIPAA relies on the Knowledge Base's built-in copy of the rule. If a fetch fails, the AI cites the Record instead.
- ❓ Do web findings get promoted into the knowledge base?

### R8 — Compliance behaviour
- ✅ Compliance and security checklists **report gaps and suggest fixes**; they never change the diagram on their own.
- ✅ Each suggested fix is a Proposal. When the user approves it, the diagram is updated.

### R10 — Diagram data model
- ✅ One `.excalidraw` file per diagram. Meaning is stored on its elements, and the graph is calculated when needed ([ADR 0001](docs/adr/0001-diagram-semantics-live-in-excalidraw-scene.md)).
- ✅ Two type layers: provider-neutral **Component Kinds** and provider-specific **Services** from Cloud Catalogs.
- ✅ Production Edges have an interaction style (sync/async/stream) and a protocol. Nodes that hold or receive data carry a multi-select Data Classification: `PHI`, `PII`, `confidential` or `public`.
- ✅ Kind IDs are `<category>.<name>`, across 7 categories: compute, data, network, security, messaging, observability and external. Group kinds are neutral: region, zone, network, subnet (public/private/isolated), account and boundary.
- ✅ Production Group membership is explicit, and a mismatch with the drawing is flagged. Rough Draft membership comes from position.
- ✅ The AI reads a compact text format with stable short aliases. Token cost is to be measured.

### R9 — AI runtime
- ✅ Same approach as px0: the AI work is **handed off to a local CLI coding agent** (e.g. Claude Code) already installed on the user's machine.
- ✅ **No API keys** are handled by the app. Login, the model and web search come from the CLI agent.
- ✅ v1 supports **Claude Code** only. Other agents could be added later behind an adapter.
- ✅ The runtime runs an **MCP server** that the agent uses to read the canvas, Design Brief and Knowledge Base, and to submit Proposals.
- ✅ **Token efficiency is a design target**: the agent reads a compact view of the diagram (not raw Excalidraw JSON), small context is put directly in the prompt, larger data is fetched on demand through tools, and one long-running agent session is reused so prompt caching applies. **Measured and approved** (token spike): the compact format is about 49× smaller than raw JSON. Only changes are sent after the first turn. 6 tools: `get_diagram`, `get_nodes`, `search_kb`, `get_record`, `submit_proposal`, `submit_gap_report`. The default system prompt is replaced (−38% input).
- ✅ **Agent session setup** ([How is the Claude Code session configured?](wayfinder/tickets/013-how-is-the-claude-code-session-configured.md)):
  - **System prompt:** static and shipped in the binary. Project context travels in messages.
  - **Isolation:** the session ignores the user's own Claude Code config (`CLAUDE.md`, plugins, hooks, slash commands) and runs in an empty working directory owned by the runtime.
  - **MCP:** Streamable HTTP on localhost, with a bearer token generated per process.
  - **Lifecycle:** starts when the Assistant sidebar opens, stops after 15 min idle, and resumes with `--resume`. After a crash it resumes once, then starts a fresh session.
  - **Model:** a per-install setting (default: Claude Code's own default), switchable mid-session.
  - **Version check:** the Claude Code version is checked against a tested range. The `--bare`/API-key risk is accepted.

### R11 — Document Pipeline
_(Being specified on its own planning map: [wayfinder/doc-pipeline/map.md](wayfinder/doc-pipeline/map.md).)_
- ✅ From the Rough Draft, Design Brief and Production Diagram, Designer produces the **standard documents a senior software engineer is expected to write**, each built by a **Step** on the outputs of earlier Steps.
- ✅ First documents: **Requirements spec, Design doc / RFC with ADRs, Threat model (STRIDE), Reliability & DR**. The arc42 export stays and should share their sources.
- ✅ **Audit trail:** one shared, append-only **Register** per Project of requirements (REQ), assumptions (ASM), constraints (CON), non-goals and out of scope (NG), decisions (DEC), risks (RISK) and open questions (Q), each with its source, the Step that introduced it and its status. Every document version cites the entries and input versions it used.
- ✅ **Missing information:** a short wizard per Step first (`unknown` allowed, recorded as an assumption), then the Assistant asks in chat, and diagram gaps become Proposals.
- ✅ **Register format** ([What is the Register format?](wayfinder/doc-pipeline/tickets/001-what-is-the-register-format.md)): an append-only, hash-chained event log in `register/log.jsonl`; types REQ, ASM, CON, NG, DEC, RISK, Q; runtime-assigned IDs; the Assistant proposes, only the user confirms or rejects; the Brief stays its own source and Rationale assumptions and trade-offs move into the Register.
- ✅ **Steps** are built-in YAML with token-free derivers; order Brief → Requirements → (Conversion) → Design doc, Threat model, Reliability & DR → arc42; documents record what they were built from and go stale, never regenerating by themselves; one `submit_step` tool.
- ✅ **Gaps:** a token-free pre-check, then a per-Step wizard (answers are confirmed entries, `unknown` becomes an open assumption), open questions recorded as `Q` entries instead of live questions, chat entries only through an accepted review card, diagram gaps as `RISK` entries with Draft a fix.
- ✅ **Documents** are written once as `documents/<step>/vNNN.json` plus a rendered Markdown copy; ADRs are rendered `DEC` entries (MADR 4.0); a document pack zip is the audit hand-off.
- ✅ **Contents** of the Requirements spec (ISO 29148 outline), Design doc and ADRs, Threat model (STRIDE-per-element, Threat Modeling Manifesto) and Reliability & DR (SLOs, failure modes, NIST 800-34 plan): see tickets 004–007 on the map.
- ❓ The Documents panel, Step wizard and Register view: prototype ticket 010.

## Product shape (decided)
- ✅ **Solo-first.** One user per install. Avoid choices that would block adding teams later.
- ✅ Runtime written in **Go** (single static binary, frontend and Knowledge Base built in). Rust was considered, but this workload (local server, launching the agent process, file I/O) would see no noticeable speed gain, and Go has the official MCP SDK and the px0 precedent.
- ✅ Frontend: React + Excalidraw, built with Vite, embedded in the binary.
- ✅ Projects are stored as plain files (JSON/markdown) in a project folder (`designer ~/my-system`), so they can be committed to git and diffed.
- ✅ **Local only**, modelled on [px0](https://github.com/px0-ai/px0): a lightweight, fast runtime (single binary) installed with a one-line shell install (`curl … | sh`). You run it locally and it serves the UI in your browser. No hosted backend.
- ✅ The goal of the current planning effort is an **implementation-ready v1 spec**, plus a pilot of about 5 knowledge base records to test the record format.

## Open scoping questions (not yet decided)
_(Tracked as tickets on the planning map: [wayfinder/map.md](wayfinder/map.md).)_

## Out of scope
- Infrastructure-as-code (Terraform/CDK) generation (v1)
- Team collaboration and real-time co-editing (v1)
- Hosted/SaaS deployment (v1)
- Writing the full knowledge base (this planning effort only pilots the record format)
- New Rough Drafts in an existing Project (e.g. sketching an extension and converting it as a Proposal): **v2**
- CLI agents other than Claude Code (v1)
- GCP target (v1; the data catalog leaves room for it)
- CIS Controls and Benchmarks (licence is NonCommercial-NoDerivatives)
