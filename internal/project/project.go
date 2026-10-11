// Package project reads and writes a Project folder: plain files that can be
// committed to git and diffed.
//
//	project.yaml              name, created, when the Rough Draft was frozen
//	brief.yaml                the Design Brief
//	rough-draft.excalidraw    the Rough Draft
//	production/vNNN.excalidraw  one file per Production Diagram version
//	production/vNNN.json        that version's meta and Design Rationale snapshot
//	gap-reports/vNNN-<time>.json  Gap Reports, never edited after they are written
//	documents/vNNN-arc42.json   the Assistant's text for that version's arc42 document
//	.designer/                runtime state (Assistant Session, Proposals); not for git
package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vinaayakh/designer/internal/brief"
	"gopkg.in/yaml.v3"
)

const (
	fileProject = "project.yaml"
	fileBrief   = "brief.yaml"
	fileDraft   = "rough-draft.excalidraw"
	dirProd     = "production"
	dirReports  = "gap-reports"
	dirDocs     = "documents"
	dirState    = ".designer"
)

// EmptyScene is a new, blank .excalidraw file.
const EmptyScene = `{"type":"excalidraw","version":2,"source":"designer","elements":[],"appState":{},"files":{}}`

type Meta struct {
	Name     string     `yaml:"name" json:"name"`
	Created  time.Time  `yaml:"created" json:"created"`
	FrozenAt *time.Time `yaml:"frozen_at,omitempty" json:"frozenAt,omitempty"`
}

type Project struct {
	Dir string
	mu  sync.Mutex
}

var ErrFrozen = errors.New("the Rough Draft is frozen")

// Open opens a Project folder, creating it (and its files) if needed.
func Open(dir string) (*Project, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	p := &Project{Dir: abs}
	if err := os.MkdirAll(p.path(dirState), 0o755); err != nil {
		return nil, err
	}
	if _, err := os.Stat(p.path(fileProject)); errors.Is(err, os.ErrNotExist) {
		m := Meta{Name: filepath.Base(abs), Created: time.Now().UTC().Truncate(time.Second)}
		if err := p.writeYAML(fileProject, m); err != nil {
			return nil, err
		}
	}
	if _, err := os.Stat(p.path(fileBrief)); errors.Is(err, os.ErrNotExist) {
		if err := p.write(fileBrief, brief.New().Marshal()); err != nil {
			return nil, err
		}
	}
	if _, err := os.Stat(p.path(fileDraft)); errors.Is(err, os.ErrNotExist) {
		if err := p.write(fileDraft, []byte(EmptyScene)); err != nil {
			return nil, err
		}
	}
	gi := p.path(dirState, ".gitignore")
	if _, err := os.Stat(gi); errors.Is(err, os.ErrNotExist) {
		_ = os.WriteFile(gi, []byte("*\n"), 0o644)
	}
	return p, nil
}

func (p *Project) path(parts ...string) string {
	return filepath.Join(append([]string{p.Dir}, parts...)...)
}

// StateDir is the runtime-owned folder inside the Project.
func (p *Project) StateDir() string { return p.path(dirState) }

func (p *Project) write(rel string, data []byte) error {
	full := p.path(rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	tmp := full + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, full)
}

func (p *Project) writeYAML(rel string, v any) error {
	b, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	return p.write(rel, b)
}

func (p *Project) Meta() (Meta, error) {
	var m Meta
	b, err := os.ReadFile(p.path(fileProject))
	if err != nil {
		return m, err
	}
	return m, yaml.Unmarshal(b, &m)
}

func (p *Project) SetName(name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	m, err := p.Meta()
	if err != nil {
		return err
	}
	m.Name = name
	return p.writeYAML(fileProject, m)
}

// ---- Brief

func (p *Project) Brief() (*brief.Brief, error) {
	b, err := os.ReadFile(p.path(fileBrief))
	if err != nil {
		return nil, err
	}
	return brief.Parse(b)
}

func (p *Project) SaveBrief(b *brief.Brief) error {
	if err := b.Validate(); err != nil {
		return err
	}
	return p.write(fileBrief, b.Marshal())
}

// ---- Rough Draft

func (p *Project) RoughDraft() ([]byte, error) { return os.ReadFile(p.path(fileDraft)) }

func (p *Project) SaveRoughDraft(scene []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	m, err := p.Meta()
	if err != nil {
		return err
	}
	if m.FrozenAt != nil {
		return ErrFrozen
	}
	if !json.Valid(scene) {
		return fmt.Errorf("rough draft: invalid JSON")
	}
	return p.write(fileDraft, scene)
}

// Freeze makes the Rough Draft read-only (on accepting the Conversion).
func (p *Project) Freeze() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	m, err := p.Meta()
	if err != nil {
		return err
	}
	now := time.Now().UTC().Truncate(time.Second)
	m.FrozenAt = &now
	return p.writeYAML(fileProject, m)
}

// ---- Production Diagram versions

type Version struct {
	N         int             `json:"n"`
	Created   time.Time       `json:"created"`
	Source    string          `json:"source"` // conversion | proposal | edit | tidy | restore
	Summary   string          `json:"summary,omitempty"`
	Proposal  string          `json:"proposal,omitempty"`
	Rationale json.RawMessage `json:"rationale,omitempty"`
}

var verRe = regexp.MustCompile(`^v(\d{3,})\.json$`)

func verName(n int) string { return fmt.Sprintf("v%03d", n) }

func (p *Project) Versions() ([]Version, error) {
	entries, err := os.ReadDir(p.path(dirProd))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Version
	for _, e := range entries {
		if !verRe.MatchString(e.Name()) {
			continue
		}
		b, err := os.ReadFile(p.path(dirProd, e.Name()))
		if err != nil {
			return nil, err
		}
		var v Version
		if err := json.Unmarshal(b, &v); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		v.Rationale = nil // listed without the snapshot
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].N < out[j].N })
	return out, nil
}

func (p *Project) Latest() (int, error) {
	vs, err := p.Versions()
	if err != nil || len(vs) == 0 {
		return 0, err
	}
	return vs[len(vs)-1].N, nil
}

func (p *Project) Version(n int) (Version, []byte, error) {
	var v Version
	b, err := os.ReadFile(p.path(dirProd, verName(n)+".json"))
	if err != nil {
		return v, nil, err
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return v, nil, err
	}
	scene, err := os.ReadFile(p.path(dirProd, verName(n)+".excalidraw"))
	return v, scene, err
}

// coalesceWindow: hand edits within this window of the previous hand-edit
// version update it instead of creating a new version per keystroke.
const coalesceWindow = 10 * time.Minute

// SaveVersion writes a new Production version. Hand edits ("edit") are
// coalesced into the latest version when it is also a recent hand edit that
// no Gap Report has checked. rationale nil carries the previous snapshot forward.
func (p *Project) SaveVersion(scene []byte, source, summary, proposal string, rationale json.RawMessage) (Version, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !json.Valid(scene) {
		return Version{}, fmt.Errorf("version: invalid scene JSON")
	}
	latest, err := p.Latest()
	if err != nil {
		return Version{}, err
	}
	now := time.Now().UTC()
	if latest > 0 {
		prev, _, err := p.Version(latest)
		if err != nil {
			return Version{}, err
		}
		if rationale == nil {
			rationale = prev.Rationale
		}
		if source == "edit" && prev.Source == "edit" && now.Sub(prev.Created) < coalesceWindow && !p.checked(latest) {
			prev.Rationale = rationale
			return prev, p.writeVersion(prev, scene)
		}
	}
	v := Version{N: latest + 1, Created: now.Truncate(time.Second), Source: source, Summary: summary, Proposal: proposal, Rationale: rationale}
	return v, p.writeVersion(v, scene)
}

func (p *Project) writeVersion(v Version, scene []byte) error {
	if err := p.write(filepath.Join(dirProd, verName(v.N)+".excalidraw"), scene); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(v, "", "  ")
	return p.write(filepath.Join(dirProd, verName(v.N)+".json"), b)
}

// SetRationale replaces the Rationale snapshot of a version.
func (p *Project) SetRationale(n int, rationale json.RawMessage) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	v, scene, err := p.Version(n)
	if err != nil {
		return err
	}
	v.Rationale = rationale
	return p.writeVersion(v, scene)
}

// ---- arc42 document text

// SaveDocText writes the document text of version n (replacing an older one).
func (p *Project) SaveDocText(n int, data []byte) error {
	return p.write(filepath.Join(dirDocs, verName(n)+"-arc42.json"), data)
}

// DocText reads the document text of version n; nil if there is none.
func (p *Project) DocText(n int) ([]byte, error) {
	b, err := os.ReadFile(p.path(dirDocs, verName(n)+"-arc42.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return b, err
}

// ---- Gap Reports

type ReportRef struct {
	File    string    `json:"file"`
	Version int       `json:"version"`
	Created time.Time `json:"created"`
}

func (p *Project) checked(n int) bool {
	refs, _ := p.Reports()
	for _, r := range refs {
		if r.Version == n {
			return true
		}
	}
	return false
}

// SaveReport writes a Gap Report for version n. Reports are never rewritten.
func (p *Project) SaveReport(n int, report json.RawMessage) (ReportRef, error) {
	now := time.Now().UTC()
	name := fmt.Sprintf("%s-%s.json", verName(n), now.Format("20060102T150405"))
	ref := ReportRef{File: name, Version: n, Created: now.Truncate(time.Second)}
	var pretty map[string]any
	if err := json.Unmarshal(report, &pretty); err != nil {
		return ref, err
	}
	b, _ := json.MarshalIndent(pretty, "", "  ")
	return ref, p.write(filepath.Join(dirReports, name), b)
}

func (p *Project) Reports() ([]ReportRef, error) {
	entries, err := os.ReadDir(p.path(dirReports))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []ReportRef
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".json") || len(name) < 5 {
			continue
		}
		vs, ts, ok := strings.Cut(strings.TrimSuffix(name, ".json"), "-")
		if !ok {
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(vs, "v"))
		if err != nil {
			continue
		}
		t, _ := time.Parse("20060102T150405", ts)
		out = append(out, ReportRef{File: name, Version: n, Created: t})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created.Before(out[j].Created) })
	return out, nil
}

func (p *Project) Report(file string) (json.RawMessage, error) {
	if filepath.Base(file) != file {
		return nil, fmt.Errorf("bad report name")
	}
	return os.ReadFile(p.path(dirReports, file))
}

// ---- Runtime state (.designer/)

func (p *Project) ReadState(name string, v any) error {
	b, err := os.ReadFile(p.path(dirState, name))
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func (p *Project) WriteState(name string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return p.write(filepath.Join(dirState, name), b)
}

func (p *Project) RemoveState(name string) error {
	err := os.Remove(p.path(dirState, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
