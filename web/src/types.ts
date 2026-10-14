// Shapes of the runtime's JSON (see internal/*). Kept in one place.

export type View = "rough" | "production";

export interface BriefValue { id: string; label: string; hint?: string }
export interface BriefField {
  id: string; short?: string; label: string; question: string; help?: string;
  required?: boolean; multi?: boolean; exclusive?: string; screen?: string; values: BriefValue[];
}
export interface BriefSchema { version: number; fields: BriefField[]; notes: { label: string; question: string; max: number } }
export interface Brief { answers: Record<string, string>; compliance: string[] | null; notes: string }

export interface Kind { id: string; gloss: string; icon: string; holdsData?: boolean }
export interface GroupKind { id: string; gloss: string; icon: string; tiers?: string[] }
export interface Service { id: string; name: string; kind: string; icon?: string; hipaa: boolean; docs?: string; aliases?: string[] }
export interface Cloud { provider: string; display: string; groupNames: Record<string, string>; hipaaAsOf: string | null; services: Service[] }
export interface Catalog { categories: string[]; kinds: Kind[]; groupKinds: GroupKind[]; dataClasses: string[]; clouds: Record<string, Cloud> }

export interface GNode { id: string; label: string; kind?: string; service?: string; serviceName?: string; classes?: string[]; attrs?: Record<string, string>; parent?: string }
export interface GGroup { id: string; label: string; kind?: string; tier?: string; parent?: string }
export interface GEdge { id: string; from: string; to: string; label?: string; style?: string; protocol?: string }
export interface GNote { id: string; text: string; near?: string; loose?: boolean }
export interface Issue { code: string; ids: string[]; message: string }
export interface Graph { production: boolean; nodes: GNode[]; groups: GGroup[]; edges: GEdge[]; notes: GNote[] | null; issues: Issue[] | null }

export interface Change {
  id: string; op: string; ref?: string; label?: string; kind?: string; service?: string; service_name?: string;
  classes?: string[]; tier?: string; parent?: string | null; from?: string; to?: string; style?: string; protocol?: string;
  set?: Record<string, unknown>; why?: string; practices?: string[];
  elementId?: string; parentId?: string | null; fromId?: string; toId?: string; needs?: string[];
}
export interface Citation { record?: string; url?: string; title?: string }
export interface NodeRationale { why: string; practices?: string[]; cites?: Citation[] }
export interface Rationale {
  summary: string; assumptions?: string[]; nodes?: Record<string, NodeRationale>;
  tradeoffs?: { decision: string; chose: string; over: string; because: string }[];
}
export interface Payload {
  title: string; summary: string; kind?: string; view?: View; base_version?: number; addresses?: string[];
  changes: Change[]; rationale?: Rationale;
}
export interface Pending {
  id: string; revision: number; kind: "proposal" | "conversion"; view: View; baseVersion: number;
  payload: Payload; created: string; awaitingRevision?: boolean; reply?: string;
  accepted?: string[]; rejections?: { change: Change; reason?: string }[]; question?: string; answers?: string[];
  /** "user" for changes applied from Sim Mode; absent for the Assistant's. */
  author?: "user"; source?: string;
}

export interface Version { n: number; created: string; source: string; summary?: string; proposal?: string; rationale?: Rationale }
export interface ReportRef { file: string; version: number; created: string }
export interface GapItem { record: string; title: string; regime?: string; coverage: string; status: "met" | "unmet" | "outside"; nodes?: string[]; finding: string; fix?: string[]; byRuntime?: boolean }
export interface GapRec { practice: string; title: string; nodes?: string[]; finding: string }
export interface GapReport { version: number; kbVersion: string; regimes: string[]; created: string; items: GapItem[]; recommendations: GapRec[] | null }

export interface KBSource { title: string; url: string; asOf: string; licenceTier: string; attribution?: string }
export interface KBRecord {
  id: string; type: string; domain?: string; regime?: string; ref?: string; obligation?: string; title: string; summary: string;
  coverage: string; satisfiedBy?: string[]; satisfies?: string[]; sources: KBSource[]; revised: string; body: string;
}

export interface AssistantStatus { running: boolean; busy: boolean; task?: string; costUsd: number; claudeVersion?: string; warning?: string; canRetry: boolean }
export interface Settings { model: string; conversionModel: string }

// ---- Document Pipeline (internal/pipeline, internal/register)

export type EntryType = "REQ" | "ASM" | "CON" | "NG" | "DEC" | "RISK" | "Q";
export type EntryStatus = "open" | "confirmed" | "rejected" | "superseded";
export interface RegisterEntry {
  id: string; type: EntryType; statement: string; fields?: Record<string, string>;
  source: { kind: string; ref?: string; text?: string }; links?: string[]; status: EntryStatus;
  step?: string; doc?: string; created: string; createdBy: string; supersedes?: string; supersededBy?: string;
  history: { at: string; actor: string; status: EntryStatus; reason?: string }[];
  citedBy?: string[];
}
export interface StepQuestion { id: string; label: string; question: string; kind: "text" | "choice" | "multi" | "list"; values?: string[]; blocking?: boolean; creates?: EntryType }
export interface StepAnswer { values?: string[]; unknown?: boolean; at: string; entries?: string[] }
export interface StepStatus { kind: "waiting" | "needs" | "ready" | "writing" | "current" | "stale"; text: string; reasons?: string[]; unanswered: number; blocking?: string[] }
export interface DocVersionInfo { step: string; n: number; created: string; model?: string; inputs: { production?: number; upstream?: Record<string, number> }; cites: Record<string, EntryStatus>; newEntries?: string[] }
export interface PipelineStep {
  id: string; title: string; version: number; template: string; credit?: string; diagram: "rough" | "production";
  requires?: string[]; uses?: string[]; questions: StepQuestion[];
  status: StepStatus; answers?: Record<string, StepAnswer>; versions: number[] | null; latest?: DocVersionInfo;
}
export interface ProposedEntry { id: string; created: string; replaces?: string; entry: { type: EntryType; statement: string; fields?: Record<string, string>; links?: string[]; source: { kind: string; text?: string } } }
export interface PipelineView { steps: PipelineStep[]; entries: RegisterEntry[] | null; proposed?: ProposedEntry[]; arc42?: { version: number; written: boolean; stale: boolean }; integrity?: string }

export interface State {
  project: { name: string; dir: string; created: string; frozenAt?: string | null };
  brief: Brief; briefDone: boolean; briefSchema: BriefSchema; catalog: Catalog; kbVersion: string;
  versions: Version[] | null; reports: ReportRef[] | null; pending: Pending | null; assistant: AssistantStatus;
  settings: Settings; trusted: string[]; view: View;
}

/** What Designer stores in element.customData.designer (internal/scene Meta). */
export interface Meta {
  role?: "node" | "group" | "edge" | "icon" | "label" | "decor";
  kind?: string; service?: string; serviceName?: string; classes?: string[]; attrs?: Record<string, string>;
  parent?: string; tier?: string; style?: string; protocol?: string; node?: string; label?: string;
}
