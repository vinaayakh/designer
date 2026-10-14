package catalog

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Behaviour Profiles: the numbers Simulation runs on, one file per Component
// Kind (sim ticket 004). The engine runs in the browser, so they are served
// as plain values; Go only loads and checks them.
//
//go:embed profiles/*.yaml
var profileFiles embed.FS

var loadProfiles = sync.OnceValues(func() (map[string]any, error) {
	out := map[string]any{}
	entries, err := fs.ReadDir(profileFiles, "profiles")
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		b, err := profileFiles.ReadFile("profiles/" + e.Name())
		if err != nil {
			return nil, err
		}
		var p map[string]any
		if err := yaml.Unmarshal(b, &p); err != nil {
			return nil, fmt.Errorf("profiles/%s: %w", e.Name(), err)
		}
		kind, _ := p["kind"].(string)
		if kind+".yaml" != e.Name() {
			return nil, fmt.Errorf("profiles/%s: kind %q doesn't match the file name", e.Name(), kind)
		}
		out[kind] = p
	}
	return out, nil
})

// Profiles returns every Behaviour Profile keyed by Kind.
func Profiles() (map[string]any, error) { return loadProfiles() }

// lintProfile lists what's wrong with one Profile: missing sections, ranges
// not ordered low ≤ typical ≤ high, prices without a date, incomplete sources.
func lintProfile(kind string, p map[string]any) []string {
	var errs []string
	for _, k := range []string{"as_of", "unit", "capacity", "latency"} {
		if p[k] == nil {
			errs = append(errs, fmt.Sprintf("%s: missing %s", kind, k))
		}
	}
	if c, ok := p["capacity"].(map[string]any); ok && c["model"] == nil {
		errs = append(errs, kind+": capacity.model missing")
	}
	var walk func(v any, path string)
	walk = func(v any, path string) {
		m, ok := v.(map[string]any)
		if !ok {
			return
		}
		lo, okL := num(m["low"])
		ty, okT := num(m["typical"])
		hi, okH := num(m["high"])
		if okL && okT && okH {
			if lo > ty || ty > hi {
				errs = append(errs, fmt.Sprintf("%s %s: range not ordered (%v, %v, %v)", kind, path, lo, ty, hi))
			}
			return
		}
		if strings.HasSuffix(path, ".price") && m["as_of"] == nil {
			errs = append(errs, fmt.Sprintf("%s %s: price without as_of", kind, path))
		}
		for k, x := range m {
			walk(x, path+"."+k)
		}
	}
	walk(p, "")
	srcs, _ := p["sources"].([]any)
	for _, s := range srcs {
		m, _ := s.(map[string]any)
		if m["url"] == nil || m["copy"] == nil || m["used_for"] == nil {
			errs = append(errs, fmt.Sprintf("%s: source %v incomplete", kind, m["id"]))
		}
	}
	return errs
}

func num(v any) (float64, bool) {
	switch x := v.(type) {
	case int:
		return float64(x), true
	case float64:
		return x, true
	}
	return 0, false
}
