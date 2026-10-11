package app

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

// Trusted Sources: the curated allowlist shipped in the binary (R7, ticket 014).
// path, when set, restricts fetches on that host to the path prefix; such a
// host is never offered to WebSearch.
var trusted = []struct{ host, path string }{
	{"docs.aws.amazon.com", ""},
	{"learn.microsoft.com", ""},
	{"owasp.org", ""},
	{"top10.owasp.org", ""},
	{"cheatsheetseries.owasp.org", ""},
	{"nist.gov", ""},
	{"csrc.nist.gov", ""},
	{"govinfo.gov", ""},
	{"ecfr.gov", ""},
	{"federalregister.gov", ""},
	{"hhs.gov", ""},
	{"aws.amazon.com", "/compliance/"},
}

// TrustedHost reports whether a host (or a subdomain of a listed host) is trusted.
func TrustedHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, t := range trusted {
		if host == t.host || strings.HasSuffix(host, "."+t.host) {
			return true
		}
	}
	return false
}

// TrustedURL checks a full URL, including path restrictions.
func TrustedURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, t := range trusted {
		if host == t.host || strings.HasSuffix(host, "."+t.host) {
			if t.path == "" || strings.HasPrefix(u.Path, t.path) {
				return true
			}
		}
	}
	return false
}

func searchable(domain string) bool {
	domain = strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(domain, "https://"), "http://"))
	domain = strings.TrimSuffix(strings.Split(domain, "/")[0], ".")
	for _, t := range trusted {
		if t.path == "" && (domain == t.host || strings.HasSuffix(domain, "."+t.host)) {
			return true
		}
	}
	return false
}

// webFetchRules are the --allowedTools permission rules (layer one: fail closed).
func webFetchRules() []string {
	var out []string
	for _, t := range trusted {
		out = append(out, "WebFetch(domain:"+t.host+")", "WebFetch(domain:*."+t.host+")")
	}
	return out
}

// TrustedList is the domain list shown in messages and the UI.
func TrustedList() []string {
	var out []string
	for _, t := range trusted {
		out = append(out, t.host+t.path)
	}
	return out
}

// hookDecision is the PreToolUse check for WebSearch and WebFetch (layer two).
func hookDecision(tool string, input map[string]any) (allow bool, reason string) {
	switch tool {
	case "WebSearch":
		raw, ok := input["allowed_domains"].([]any)
		if !ok || len(raw) == 0 {
			return false, "WebSearch must set allowed_domains to trusted domains only: " + strings.Join(searchableList(), ", ")
		}
		for _, d := range raw {
			if !searchable(fmt.Sprint(d)) {
				return false, fmt.Sprintf("%v is not a trusted domain. Allowed: %s", d, strings.Join(searchableList(), ", "))
			}
		}
		return true, ""
	case "WebFetch":
		u, _ := input["url"].(string)
		if !TrustedURL(u) {
			return false, fmt.Sprintf("%s is not on the trusted list (%s). Cite a Knowledge Base Record instead.", u, strings.Join(TrustedList(), ", "))
		}
		return true, ""
	}
	return true, ""
}

func searchableList() []string {
	var out []string
	for _, t := range trusted {
		if t.path == "" {
			out = append(out, t.host)
		}
	}
	return out
}

// hookSettings builds the --settings JSON: an HTTP PreToolUse hook that
// blocks on failure, so the allowlist fails closed.
func hookSettings(hookURL, token string) string {
	s := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{map[string]any{
				"matcher": "WebSearch|WebFetch",
				"hooks": []any{map[string]any{
					"type": "http", "url": hookURL, "timeout": 10, "onFailure": "block",
					"headers": map[string]string{"Authorization": "Bearer " + token},
				}},
			}},
		},
	}
	b, _ := json.Marshal(s)
	return string(b)
}

// ServeHook answers Claude Code's PreToolUse HTTP hook.
func (a *App) ServeHook(w http.ResponseWriter, r *http.Request) {
	if !a.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var in struct {
		ToolName  string         `json:"tool_name"`
		ToolInput map[string]any `json:"tool_input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "bad hook input", http.StatusBadRequest)
		return
	}
	allow, reason := hookDecision(in.ToolName, in.ToolInput)
	if allow {
		slog.Debug("hook allowed a tool", "tool", in.ToolName)
	} else {
		slog.Warn("hook blocked a tool", "tool", in.ToolName, "reason", reason, "input", truncate(fmt.Sprint(in.ToolInput), 300))
	}
	if in.ToolName == "WebSearch" || in.ToolName == "WebFetch" {
		a.emit("assistant.web", map[string]any{"tool": in.ToolName, "input": in.ToolInput, "allowed": allow})
	}
	if allow {
		// An empty 2xx body leaves the decision to the permission rules.
		w.WriteHeader(http.StatusOK)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"hookSpecificOutput": map[string]any{
		"hookEventName": "PreToolUse", "permissionDecision": "deny", "permissionDecisionReason": reason,
	}})
}
