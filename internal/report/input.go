package report

import (
	"time"

	"github.com/vinaayakh/designer/internal/kb"
	"github.com/vinaayakh/designer/internal/proposal"
	"github.com/vinaayakh/designer/internal/register"
	"github.com/vinaayakh/designer/internal/scene"
)

// Input is everything the document is built from. The app fills it in.
type Input struct {
	Project   string
	Version   int
	Created   time.Time // when the version was saved
	Generated time.Time
	Graph     *scene.Graph

	Brief       []BriefRow
	Regimes     []string          // compliance regime ids from the Brief (no "none")
	RegimeNames map[string]string // id → display name, e.g. soc2 → SOC 2
	Cloud       string            // display name of the target, e.g. "AWS" or "Generic"

	Kinds      map[string]string // Component Kind id → gloss
	GroupKinds map[string]string // group kind id → gloss
	Services   map[string]Service

	Rationale *proposal.Rationale
	Entries   []register.Entry      // the Register; when present it is the source of requirements, assumptions, decisions and risks
	Gap       *Gap                  // the newest Gap Report of this version or an earlier one
	Applies   map[string][]string   // element id → Knowledge Base Records (controls and practices) that apply
	Records   map[string]*kb.Record // every Record the document mentions
	KBVersion string

	Narrative       *Narrative // nil: sections fall back to stored data only
	HIPAAUnverified bool       // the Cloud Catalog's HIPAA flags have no verification date
}

// BriefRow is one Design Brief answer, already in words.
type BriefRow struct {
	ID, Label, Value string
	Unknown          bool
}

type Service struct {
	Name  string
	Docs  string
	HIPAA bool
}

// Gap mirrors a saved Gap Report (internal/app).
type Gap struct {
	Version         int       `json:"version"`
	KBVersion       string    `json:"kbVersion"`
	Regimes         []string  `json:"regimes"`
	Created         time.Time `json:"created"`
	Items           []GapItem `json:"items"`
	Recommendations []GapRec  `json:"recommendations"`
}

type GapItem struct {
	Record    string   `json:"record"`
	Title     string   `json:"title"`
	Regime    string   `json:"regime,omitempty"`
	Coverage  string   `json:"coverage"`
	Status    string   `json:"status"` // met | unmet | outside
	Nodes     []string `json:"nodes,omitempty"`
	Finding   string   `json:"finding"`
	Fix       []string `json:"fix,omitempty"`
	ByRuntime bool     `json:"byRuntime,omitempty"`
}

type GapRec struct {
	Practice string   `json:"practice"`
	Title    string   `json:"title"`
	Nodes    []string `json:"nodes,omitempty"`
	Finding  string   `json:"finding"`
}

// Narrative is the text the Assistant writes for a version: what stored data
// can't say. Element ids, not aliases.
type Narrative struct {
	Version    int                 `json:"version"`
	Created    time.Time           `json:"created"`
	Model      string              `json:"model,omitempty"`
	Overview   string              `json:"overview"`
	Components map[string][]string `json:"components"`       // Node id → responsibilities
	Groups     map[string]string   `json:"groups,omitempty"` // Group id → purpose
	Scenarios  []Scenario          `json:"scenarios"`
	Concepts   []Concept           `json:"concepts"`
	Quality    []Quality           `json:"quality"`
}

type Scenario struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Steps   []Step `json:"steps"`
}

type Step struct {
	From string `json:"from"`
	To   string `json:"to"`
	What string `json:"what"`
}

type Concept struct {
	Topic string   `json:"topic"`
	Text  string   `json:"text"`
	Nodes []string `json:"nodes,omitempty"`
}

type Quality struct {
	Attribute   string   `json:"attribute"`
	Requirement string   `json:"requirement"`
	Scenario    string   `json:"scenario"`
	Response    string   `json:"response"`
	Nodes       []string `json:"nodes,omitempty"`
}

// Stale reports whether a Narrative no longer matches the diagram: a Node
// without responsibilities, or text about elements that are gone.
func (n *Narrative) Stale(g *scene.Graph) bool {
	if n == nil {
		return true
	}
	for _, nd := range g.Nodes {
		if len(n.Components[nd.ID]) == 0 {
			return true
		}
	}
	for id := range n.Components {
		if g.Node(id) == nil {
			return true
		}
	}
	for _, s := range n.Scenarios {
		for _, st := range s.Steps {
			if g.Node(st.From) == nil || g.Node(st.To) == nil {
				return true
			}
		}
	}
	return false
}
