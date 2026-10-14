package project

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// Scenarios are Sim Mode's working copies of a Production version: a change
// list plus traffic, kept as plain YAML so they diff well (sim ticket 006).
// The browser owns their shape; the runtime stores them.
const dirScenarios = "sim/scenarios"

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

var ErrBadSlug = errors.New("a Scenario name is lowercase letters, digits and dashes")

// Scenarios lists the saved Scenario slugs.
func (p *Project) Scenarios() ([]string, error) {
	es, err := os.ReadDir(p.path(dirScenarios))
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, e := range es {
		if s, ok := strings.CutSuffix(e.Name(), ".yaml"); ok && slugRe.MatchString(s) {
			out = append(out, s)
		}
	}
	slices.Sort(out)
	return out, nil
}

// Scenario reads one Scenario. A missing one returns os.ErrNotExist.
func (p *Project) Scenario(slug string) (map[string]any, error) {
	if !slugRe.MatchString(slug) {
		return nil, ErrBadSlug
	}
	b, err := os.ReadFile(p.path(filepath.FromSlash(dirScenarios), slug+".yaml"))
	if err != nil {
		return nil, err
	}
	var v map[string]any
	if err := yaml.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	return v, nil
}

// SaveScenario writes one Scenario.
func (p *Project) SaveScenario(slug string, v map[string]any) error {
	if !slugRe.MatchString(slug) {
		return ErrBadSlug
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.writeYAML(filepath.Join(filepath.FromSlash(dirScenarios), slug+".yaml"), v)
}
