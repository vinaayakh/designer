# Glossary

**Project**: One system being designed. It holds a Design Brief, one Rough Draft, and zero or more Production Diagrams.

**Design Brief**: The non-functional targets captured when a Project is created: CAP trade-off, user scale, performance expectations, read/write bias, target cloud, compliance regimes, and so on. The AI treats it as a standing constraint.

**Rough Draft**: The hand-drawn, back-of-the-napkin sketch that starts a Project. It is editable until the first Conversion, then frozen as a read-only reference.

**Node**: A single element on the Rough Draft, such as a box or shape, that represents a component of the system. It can be added to the Assistant's message context.

**Conversion**: The AI step that turns the current Rough Draft, together with the Design Brief and Knowledge Base, into a new Production Diagram.

**Production Diagram**: The polished, cloud-specific diagram produced by Conversion. Once it exists it is the Project's working source: fully editable, with every edit saving a new version. It comes with a Design Rationale and a Gap Report.

**Design Rationale**: The explanation attached to a Production Diagram: why each component exists, which practices it applies, and the trade-offs made.

**Gap Report**: A list attached to a Production Diagram showing which items on the selected compliance and security checklists are met, unmet, or outside what the diagram can show. Unmet items come with suggested fixes as Proposals. The report itself never changes the diagram.

**Assistant**: The AI helper in the sidebar, available in both the Rough Draft and Production Diagram views. It can read the canvas and make Proposals. Its work is done by the user's local CLI Agent.

**CLI Agent**: A coding-agent command-line tool already on the user's machine (e.g. Claude Code) that does the AI work. It brings its own login, model and web search.

**Proposal**: A set of changes to the active diagram (Rough Draft or Production Diagram) suggested by the Assistant. The user accepts it, rejects it, or replies with changes. Nothing is applied without the user's approval.

**Knowledge Base**: The curated set of Records the AI consults before anything else.

**Record**: One entry in the Knowledge Base: a design practice, security practice, or compliance checklist item, with the sources it is based on.

**Trusted Source**: An external source the AI may search when the Knowledge Base has no answer.
