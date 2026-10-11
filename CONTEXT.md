# Glossary

**Project**: One system being designed. It holds a Design Brief, one Rough Draft, and zero or more Production Diagrams.

**Design Brief**: The non-functional targets captured by the setup wizard when a Project is created: CAP trade-off (CP or AP), user scale, performance expectations, read/write bias, target cloud, compliance regimes, and so on, plus optional free-text notes. Target cloud, compliance and CAP are required. Any other answer can be **unknown**, and the AI states what it assumed instead. Choosing no compliance regime ("none") is an answer, not an unknown. The AI treats the Brief as a standing constraint.

**Rough Draft**: The hand-drawn, back-of-the-napkin sketch that starts a Project. It is editable until the Design from Conversion is accepted, then frozen as a read-only reference.

**Node**: A shape on a diagram (rectangle, ellipse, diamond, image) that stands for one component of the system. On the Rough Draft it may have no type, and its label carries the meaning. On a Production Diagram it always has a Component Kind and, for AWS/Azure targets, a Service. Nodes that store data carry a Data Classification. It can be added to the Assistant's message context.

**Edge**: An arrow connecting two Nodes. On the Rough Draft it has direction and an optional label. On a Production Diagram it also has an interaction style (sync / async / stream) and an optional protocol.

**Note**: Free text, or an arrow not connected at both ends, on a diagram. It is passed to the AI as an annotation (e.g. "~10k rps peak"). It is not a component.

**Group**: A shape that contains other Nodes. Groups can nest. Each has a provider-neutral **group kind**: region, zone, network (shown as VPC on AWS, VNet on Azure), subnet (public, private or isolated), account (an AWS account or Azure subscription), or boundary (a trust zone such as on-prem or a third party). On the Rough Draft, membership is worked out from what is drawn inside what. On a Production Diagram, membership is an explicit parent link.

**Alias**: A short handle (`n1`, `g2`) the AI uses for a Node or Group. It stays the same for a whole Assistant Session.

**Component Kind**: A provider-neutral category of component from a fixed vocabulary (e.g. relational database, queue, CDN). Every Kind belongs to one **category**: compute, data, network, security, messaging, observability or external. Knowledge Base Records can attach to Kinds or to a whole category.

**Service**: A provider-specific offering in a Cloud Catalog (e.g. Amazon RDS for PostgreSQL) that maps to exactly one Component Kind. A product that offers several engines or APIs (e.g. Cosmos DB) is several Services. A component with no catalog entry, such as a third-party SaaS, is an **off-catalog service**: a Kind plus the service's name, shown with the generic icon.

**Cloud Catalog**: The list of Services for one target cloud (AWS, Azure). The Generic target uses Component Kinds only.

**Data Classification**: One or more labels on a Node that holds or receives data (a data store, a queue or stream, logs, or an external service it is sent to), saying what kind of data it holds: PHI, PII, confidential or public. A Node with none is **unclassified**, and the AI states what it assumed. It drives compliance checks.

**Conversion**: The one-time AI step that turns the Rough Draft, together with the Design Brief and Knowledge Base, into the Project's first Production Diagram. It runs as two tasks: **Design**, then **Gap Check**. It happens once per Project; after that, every change comes through Proposals.

**Design**: The first task of Conversion. The AI types each Node (Component Kind, Service), designs the Production Diagram and writes its Design Rationale. The user reviews the result as a whole in the **Conversion Preview** and accepts it, replies (producing a new revision) or rejects it. Accepting creates Production version 1 and freezes the Rough Draft.

**Gap Check**: The task that produces a Gap Report for one Production Diagram version. It runs automatically after Design is accepted and can be run again on any later version.

**Production Diagram**: The polished, cloud-specific diagram produced by Conversion. Once it exists it is the Project's working source: fully editable, with every edit saving a new version. It comes with a Design Rationale and a Gap Report.

**Design Rationale**: The explanation attached to a Production Diagram: why each Node exists and which Practices it applies, the Assumptions the AI made about the Rough Draft, and the trade-offs. Accepted Proposals keep it current.

**Gap Report**: The result of a Gap Check for one Production Diagram version. It says, for each Control that applies, whether it is met, unmet, or outside what the diagram can show, and has a separate section of security recommendations. Unmet items can be turned into a Proposal on request (Fix). The report never changes the diagram, and is never edited after it's written.

**Assistant**: The AI helper in the sidebar, available in both the Rough Draft and Production Diagram views. It can read the canvas and make Proposals. Its work is done by the user's local CLI Agent.

**Assistant Session**: One continuous conversation between the Assistant and a Project, during which Aliases stay stable and the Assistant remembers earlier turns. A session can end (idle, restart, crash). The next one starts from the Project's current state and has no memory of earlier chat.

**CLI Agent**: A coding-agent command-line tool already on the user's machine (e.g. Claude Code) that does the AI work. It brings its own login, model and web search.

**Proposal**: A set of changes to the active diagram (Rough Draft or Production Diagram) suggested by the Assistant. The user accepts or skips each change, accepts everything at once, rejects it, or replies with changes, which produces a new **revision** of the same Proposal. Everything accepted is saved as one new diagram version. Nothing is applied without the user's approval, and the diagram can't be edited by hand while a Proposal is waiting for review.

**Knowledge Base**: The curated set of Records the AI consults before anything else.

**Record**: One entry in the Knowledge Base, either a Control or a Practice, with the sources it is based on.

**Control**: A Record for one item on a compliance checklist: one SOC 2 criterion, or one HIPAA implementation specification. Controls are what a Gap Report marks met, unmet or outside the diagram.

**Practice**: A Record for one named design or security practice (e.g. encryption at rest, cache-aside). A Control lists the Practices that satisfy it, and those become the suggested fixes.

**Coverage**: How much of a Record a diagram can show: fully (checkable), only whether the component exists (partial), or not at all because it's policy or procedure (process). Process Records always land in the Gap Report's "outside what the diagram can show".

**Licence Tier**: How freely a Record may reuse one of its sources: public domain (quote verbatim), attribution (adapt with credit), or facts-only (IDs and facts in our own words, plus a link).

**Trusted Source**: An external source the AI may search when the Knowledge Base has no answer.

**Document Pipeline**: The set of Steps that turn a Project (Design Brief, Rough Draft, Production Diagram, earlier documents) into standard engineering documents, such as the Requirements spec, Design doc, Threat model and Reliability & DR. Each document version records what it was built from.

**Step**: One stage of the Document Pipeline that produces one kind of document. It declares the inputs and the information it needs; what's missing is asked for (wizard first, then chat, or a Proposal for the diagram), and the Assistant writes the document. A document whose inputs have changed since it was written is **stale**.

**Register**: The Project's shared, append-only audit trail. Its **entries** are requirements (REQ), assumptions (ASM), constraints (CON), non-goals and out of scope (NG), decisions (DEC), risks (RISK) and open questions (Q). An entry's text never changes; a correction is a new entry that **supersedes** it. Each records where it came from, the Step that introduced it and its status (open, confirmed, rejected or superseded); documents cite the entries they rely on. An `unknown` answer becomes an assumption here, never a silent default.
