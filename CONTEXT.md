# Glossary

**Project**: One system being designed. It holds a Design Brief, one Rough Draft, and zero or more Production Diagrams.

**Design Brief**: The non-functional targets captured by the setup wizard when a Project is created: CAP trade-off (CP or AP), user scale, performance expectations, read/write bias, target cloud, compliance regimes, and so on. The AI treats it as a standing constraint.

**Rough Draft**: The hand-drawn, back-of-the-napkin sketch that starts a Project. It is editable until the first Conversion, then frozen as a read-only reference.

**Node**: A shape on a diagram (rectangle, ellipse, diamond, image) that stands for one component of the system. On the Rough Draft it may have no type, and its label carries the meaning. On a Production Diagram it always has a Component Kind and, for AWS/Azure targets, a Service. Nodes that store data carry a Data Classification. It can be added to the Assistant's message context.

**Edge**: An arrow connecting two Nodes. On the Rough Draft it has direction and an optional label. On a Production Diagram it also has an interaction style (sync / async / stream) and an optional protocol.

**Note**: Free text, or an arrow not connected at both ends, on a diagram. It is passed to the AI as an annotation (e.g. "~10k rps peak"). It is not a component.

**Group**: A shape that contains other Nodes, e.g. Region, VPC/VNet, Subnet, Availability Zone. Groups can nest. On the Rough Draft, membership is worked out from what is drawn inside what. On a Production Diagram, membership is an explicit parent link.

**Alias**: A short handle (`n1`, `g2`) the AI uses for a Node or Group. It stays the same for a whole Assistant Session.

**Component Kind**: A provider-neutral category of component from a fixed vocabulary (e.g. relational database, queue, CDN). Knowledge Base Records can attach to Kinds.

**Service**: A provider-specific offering in a Cloud Catalog (e.g. AWS RDS for PostgreSQL) that maps to exactly one Component Kind.

**Cloud Catalog**: The list of Services for one target cloud (AWS, Azure). The Generic target uses Component Kinds only.

**Data Classification**: A label on a data-storing Node saying what kind of data it holds (e.g. PHI, PII, public). It drives compliance checks.

**Conversion**: The AI step that turns the current Rough Draft, together with the Design Brief and Knowledge Base, into a new Production Diagram.

**Production Diagram**: The polished, cloud-specific diagram produced by Conversion. Once it exists it is the Project's working source: fully editable, with every edit saving a new version. It comes with a Design Rationale and a Gap Report.

**Design Rationale**: The explanation attached to a Production Diagram: why each component exists, which practices it applies, and the trade-offs made.

**Gap Report**: A list attached to a Production Diagram showing which items on the selected compliance and security checklists are met, unmet, or outside what the diagram can show. Unmet items come with suggested fixes as Proposals. The report itself never changes the diagram.

**Assistant**: The AI helper in the sidebar, available in both the Rough Draft and Production Diagram views. It can read the canvas and make Proposals. Its work is done by the user's local CLI Agent.

**Assistant Session**: One continuous conversation between the Assistant and a Project, during which Aliases stay stable and the Assistant remembers earlier turns. A session can end (idle, restart, crash). The next one starts from the Project's current state and has no memory of earlier chat.

**CLI Agent**: A coding-agent command-line tool already on the user's machine (e.g. Claude Code) that does the AI work. It brings its own login, model and web search.

**Proposal**: A set of changes to the active diagram (Rough Draft or Production Diagram) suggested by the Assistant. The user accepts or skips each change, accepts everything at once, rejects it, or replies with changes, which produces a new **revision** of the same Proposal. Everything accepted is saved as one new diagram version. Nothing is applied without the user's approval, and the diagram can't be edited by hand while a Proposal is waiting for review.

**Knowledge Base**: The curated set of Records the AI consults before anything else.

**Record**: One entry in the Knowledge Base: a design practice, security practice, or compliance checklist item, with the sources it is based on.

**Trusted Source**: An external source the AI may search when the Knowledge Base has no answer.
