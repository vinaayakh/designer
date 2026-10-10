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
- ✅ The Rough Draft should feel like a **back-of-the-napkin sketch**: fast and loose.
- ✅ Once the first Production Diagram is created, the Rough Draft is **frozen (read-only)**. It stays as a reference for both the user and the AI.

### R2 — AI conversion to a production-ready diagram
- ✅ AI converts the rough draft into a production-ready diagram using design best practices.
- ✅ "Production-ready" output for v1 =
  - (a) a clean, laid-out diagram using the target cloud's service icons,
  - (b) a **design rationale** (why each component exists, which practices it applies, trade-offs),
  - (c) a **compliance/security gap report** against the selected checklists (SOC 2 / HIPAA).
- ✅ Each conversion produces a new, versioned **Production Diagram**. Versions can be compared.
- ✅ The Production Diagram is **fully editable** and becomes the working source once it exists. Edits save new versions.
- ❌ Infrastructure-as-code generation is not in v1.

### R3 — Cloud-provider targeting
- ✅ If a cloud provider is specified, generated components use that provider's services (e.g. AWS, Azure, GCP).
- ✅ v1 targets: **Generic** (provider-neutral), **AWS**, **Azure**.
- ✅ Provider services are kept as a data catalog, so adding providers (e.g. GCP) needs no code changes.

### R4 — AI helper sidebar
- ✅ AI assistant lives in a sidebar, available in **both** the Rough Draft view and the Production Diagram view.
- ✅ The assistant has read access to the contents of the design view.
- ✅ Clicking a node on the canvas (in either view) adds it to the context of the message being sent to the assistant.
- ✅ Proposals apply to whichever view is active. Gap Report fixes apply to the latest Production Diagram.
- ✅ The assistant can **propose** changes to the Rough Draft. The user reviews each proposal and can **accept**, **reject**, or **reply with changes** (counter-suggest), and the assistant then revises it. Nothing is applied without the user's approval.

### R5 — Project setup interview
When starting a new project, the user is asked for the system's non-functional targets, including:
- ✅ CAP theorem trade-off (which two of Consistency / Availability / Partition tolerance are prioritized).
- ✅ Expected number of users.
- ✅ Performance expectations (latency, throughput).
- ✅ Read-optimized vs write-optimized workload.
- ✅ Also: availability target, data volume and growth, RPO/RTO, regions and data residency, compliance regimes (none / SOC 2 / HIPAA), target cloud, budget sensitivity, team/ops maturity.
- ✅ Required: target cloud, compliance regimes, CAP. Everything else is optional and defaults to "unknown", which the AI states as an assumption.

### R6 — Internal knowledge base
An internal, curated database of records that the AI consults **first**:
- ✅ Software design best practices.
- ✅ Security best practices.
- ✅ Compliance checklists: **SOC 2** and **HIPAA**.
- ✅ Records are produced via deep research as part of the app development process.
- 🔬 Deep research needed to author these records from authoritative sources.
- ✅ v1: the Knowledge Base is **shipped with the app, curated, and read-only**. Users can't add their own records in v1.
- ❓ Record format/schema, retrieval method, versioning and citation of sources.

### R7 — Trusted web search fallback
- ✅ When the knowledge base lacks needed information, the AI may search the web.
- ❓ What counts as a "trusted source" (allowlist of domains? e.g. cloud provider docs, NIST, HHS, AICPA, OWASP)?
- ❓ Do web findings get promoted into the knowledge base?

### R8 — Compliance behaviour
- ✅ Compliance and security checklists **report gaps and suggest fixes**; they never change the diagram on their own.
- ✅ Each suggested fix is a Proposal. When the user approves it, the diagram is updated.

### R10 — Diagram data model
- ✅ One `.excalidraw` file per diagram. Meaning is stored on its elements, and the graph is calculated when needed ([ADR 0001](docs/adr/0001-diagram-semantics-live-in-excalidraw-scene.md)).
- ✅ Two type layers: provider-neutral **Component Kinds** and provider-specific **Services** from Cloud Catalogs.
- ✅ Production Edges have an interaction style (sync/async/stream) and a protocol. Data-storing Nodes carry a Data Classification (PHI/PII/…).
- ✅ Production Group membership is explicit, and a mismatch with the drawing is flagged. Rough Draft membership comes from position.
- ✅ The AI reads a compact text format with stable short aliases. Token cost is to be measured.

### R9 — AI runtime
- ✅ Same approach as px0: the AI work is **handed off to a local CLI coding agent** (e.g. Claude Code) already installed on the user's machine.
- ✅ **No API keys** are handled by the app. Login, the model and web search come from the CLI agent.
- ✅ v1 supports **Claude Code** only. Other agents could be added later behind an adapter.
- ✅ The runtime runs an **MCP server** that the agent uses to read the canvas, Design Brief and Knowledge Base, and to submit Proposals.
- ✅ **Token efficiency is a design target**: the agent reads a compact view of the diagram (not raw Excalidraw JSON), small context is put directly in the prompt, larger data is fetched on demand through tools, and one long-running agent session is reused so prompt caching applies. **Measured and approved** (token spike): the compact format is about 49× smaller than raw JSON. Only changes are sent after the first turn. About 6 tools. The default system prompt is replaced (−38% input).

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
