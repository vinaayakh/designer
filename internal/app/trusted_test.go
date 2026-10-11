package app

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTrustedURL(t *testing.T) {
	for url, want := range map[string]bool{
		"https://docs.aws.amazon.com/rds/latest/userguide/x.html":              true,
		"https://csrc.nist.gov/pubs/sp/800/53":                                 true,
		"https://www.nist.gov/":                                                true, // subdomain of a listed host
		"https://aws.amazon.com/compliance/hipaa-eligible-services-reference/": true,
		"https://aws.amazon.com/rds/pricing/":                                  false, // path-restricted host
		"https://evil.com/docs.aws.amazon.com":                                 false,
		"https://docs.aws.amazon.com.evil.com/":                                false,
		"https://notnist.gov/":                                                 false,
		"ftp://nist.gov/":                                                      false,
	} {
		if got := TrustedURL(url); got != want {
			t.Errorf("TrustedURL(%q) = %v, want %v", url, got, want)
		}
	}
}

func TestHookDecision(t *testing.T) {
	in := func(s string) map[string]any {
		var m map[string]any
		_ = json.Unmarshal([]byte(s), &m)
		return m
	}
	cases := []struct {
		tool, input string
		allow       bool
		reason      string
	}{
		{"WebSearch", `{"query":"x"}`, false, "must set allowed_domains"},
		{"WebSearch", `{"query":"x","allowed_domains":["owasp.org","nist.gov"]}`, true, ""},
		{"WebSearch", `{"query":"x","allowed_domains":["owasp.org","reddit.com"]}`, false, "reddit.com is not a trusted domain"},
		{"WebSearch", `{"query":"x","allowed_domains":["aws.amazon.com"]}`, false, "not a trusted domain"}, // fetch-only host
		{"WebFetch", `{"url":"https://cheatsheetseries.owasp.org/x"}`, true, ""},
		{"WebFetch", `{"url":"https://code.claude.com/docs"}`, false, "not on the trusted list"},
	}
	for _, c := range cases {
		allow, reason := hookDecision(c.tool, in(c.input))
		if allow != c.allow || !strings.Contains(reason, c.reason) {
			t.Errorf("%s %s: got (%v, %q), want (%v, …%q…)", c.tool, c.input, allow, reason, c.allow, c.reason)
		}
	}
}

func TestHookSettingsFailClosed(t *testing.T) {
	var s map[string]any
	if err := json.Unmarshal([]byte(hookSettings("http://127.0.0.1:1/hook", "tok")), &s); err != nil {
		t.Fatal(err)
	}
	h := s["hooks"].(map[string]any)["PreToolUse"].([]any)[0].(map[string]any)["hooks"].([]any)[0].(map[string]any)
	if h["onFailure"] != "block" || h["type"] != "http" {
		t.Fatalf("hook must be an HTTP hook that blocks on failure: %v", h)
	}
}
