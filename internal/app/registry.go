package app

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/vinaayakh/designer/internal/brief"
	"github.com/vinaayakh/designer/internal/proposal"
	"github.com/vinaayakh/designer/internal/register"
)

// Brief fields that state a measurable target become requirements; the rest constrain the design.
var briefTargets = map[string]bool{"users": true, "latency": true, "throughput": true, "read_write_bias": true, "availability": true, "data_volume": true, "rpo": true, "rto": true}

const noteRationaleImported = "imported the Design Rationale"

// SyncBrief keeps one live Register entry per Design Brief answer (wayfinder/doc-pipeline ticket
// 001): a known target is a confirmed REQ, any other known answer a confirmed CON, and an unknown
// answer an open ASM. A changed answer supersedes the old entry, so the history shows every change.
func (a *App) SyncBrief() error {
	b, err := a.Brief()
	if err != nil {
		return err
	}
	current := map[string]register.Entry{}
	for _, e := range a.Reg.Entries() {
		if e.Source.Kind == "brief" && e.Live() {
			current[e.Source.Ref] = e
		}
	}
	for _, f := range brief.TheSchema().Fields {
		want := briefEntry(b, f)
		if want == nil {
			continue
		}
		have, ok := current[f.ID]
		switch {
		case !ok:
			if _, err := a.Reg.Create(register.ActorRuntime, "brief", "", *want, ""); err != nil {
				return err
			}
		case have.Statement != want.Statement || have.Type != want.Type:
			if _, err := a.Reg.Supersede(register.ActorRuntime, "brief", "", have.ID, *want, "the Design Brief answer changed"); err != nil {
				return err
			}
		}
	}
	return nil
}

func briefEntry(b *brief.Brief, f brief.Field) *register.New {
	label := func(v string) string {
		for _, x := range f.Values {
			if x.ID == v {
				return x.Label
			}
		}
		return v
	}
	var val string
	if f.ID == "compliance" {
		var names []string
		for _, c := range b.Compliance {
			names = append(names, label(c))
		}
		val = strings.Join(names, ", ")
	} else {
		val = b.Get(f.ID)
	}
	if val == "" {
		return nil // required and not answered yet
	}
	src := register.Source{Kind: "brief", Ref: f.ID}
	if val == brief.Unknown {
		return &register.New{Type: register.ASM, Statement: fmt.Sprintf("%s is unknown; documents state the value they assume", f.Label), Source: src,
			Fields: map[string]string{"if_wrong": "targets and decisions that depend on " + strings.ToLower(f.Label)}, Status: register.Open}
	}
	if briefTargets[f.ID] {
		return &register.New{Type: register.REQ, Statement: fmt.Sprintf("%s: %s", f.Label, label(val)), Source: src,
			Fields: map[string]string{"kind": "non-functional", "priority": "must"}, Status: register.Confirmed}
	}
	origin := "brief"
	if f.ID == "compliance" {
		origin = "regulatory"
	}
	return &register.New{Type: register.CON, Statement: fmt.Sprintf("%s: %s", f.Label, label(val)), Source: src,
		Fields: map[string]string{"origin": origin}, Status: register.Confirmed}
}

// importRationale copies the Design Rationale's assumptions and trade-offs into the Register once,
// as open entries for the user to confirm.
func (a *App) importRationale() error {
	if a.Reg.HasNote(noteRationaleImported) {
		return nil
	}
	n, err := a.P.Latest()
	if err != nil || n == 0 {
		return err
	}
	v, _, err := a.P.Version(n)
	if err != nil || len(v.Rationale) == 0 {
		return err
	}
	var r proposal.Rationale
	if err := json.Unmarshal(v.Rationale, &r); err != nil {
		return err
	}
	src := register.Source{Kind: "import", Ref: fmt.Sprintf("Design Rationale, Production v%d", n)}
	for _, s := range r.Assumptions {
		if _, err := a.Reg.Create(register.ActorRuntime, "conversion", "", register.New{Type: register.ASM, Statement: truncate(s, 600), Source: src, Status: register.Open}, ""); err != nil {
			return err
		}
	}
	for _, t := range r.Tradeoffs {
		e := register.New{Type: register.DEC, Statement: truncate(t.Decision+": "+t.Chose, 600), Source: src, Status: register.Open,
			Fields: map[string]string{"chose": t.Chose, "over": t.Over, "because": t.Because}}
		if _, err := a.Reg.Create(register.ActorRuntime, "conversion", "", e, ""); err != nil {
			return err
		}
	}
	slog.Info("register: imported the Design Rationale", "version", n, "assumptions", len(r.Assumptions), "decisions", len(r.Tradeoffs))
	return a.Reg.Note(noteRationaleImported)
}

// openRegister loads the Register and brings it up to date with the Brief and Rationale.
func (a *App) openRegister() error {
	reg, err := register.Load(a.P.RegisterPath())
	if err != nil {
		return err
	}
	a.Reg = reg
	if msg := reg.Integrity(); msg != "" {
		slog.Warn("register: hash chain broken", "problem", msg)
	}
	if err := a.SyncBrief(); err != nil {
		return fmt.Errorf("register: %w", err)
	}
	return a.importRationale()
}
