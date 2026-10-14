package app

import (
	"slices"
	"strings"
	"testing"
)

func TestDocumentChainWritesWhatItBuildsOnFirst(t *testing.T) {
	a := pipelineApp(t)
	for step, want := range map[string][]string{
		"requirements": {"requirements"},
		"design-doc":   {"requirements", "design-doc"}, // requires the Requirements spec
		"reliability":  {"requirements", "reliability"},
		"threat-model": {"threat-model"}, // only uses it: written without it if it's missing
	} {
		got, err := a.DocumentChain(step)
		if err != nil || !slices.Equal(got, want) {
			t.Errorf("%s: %v %v, want %v", step, got, err, want)
		}
	}
	if _, err := a.DocumentChain("nope"); err == nil {
		t.Error("unknown document accepted")
	}
}

func TestCreateWizardAnswersAreCheckedTogetherThenRecorded(t *testing.T) {
	a := pipelineApp(t)
	// Every problem at once, and nothing recorded.
	before := len(a.Reg.Entries()) // the Brief's unknowns are already there
	_, err := a.prepareDocument("design-doc", []WizardAnswer{
		{Step: "requirements", Question: "problem", Unknown: true},             // required: can't be unknown
		{Step: "requirements", Question: "nope", Values: []string{"x"}},        // no such question
		{Step: "threat-model", Question: "assets", Values: []string{"Orders"}}, // not part of this document
	})
	if err == nil || strings.Count(err.Error(), ";") != 2 {
		t.Fatalf("want three problems in one error, got %v", err)
	}
	if n := len(a.Reg.Entries()); n != before {
		t.Fatalf("%d entries recorded from a rejected wizard", n)
	}
	// A required question left unanswered stops it too.
	if _, err := a.prepareDocument("design-doc", nil); err == nil || !strings.Contains(err.Error(), "needs") {
		t.Fatalf("missing required answers: %v", err)
	}
	chain, err := a.prepareDocument("design-doc", []WizardAnswer{
		{Step: "requirements", Question: "problem", Values: []string{"Customers can't track orders."}},
		{Step: "requirements", Question: "roles", Values: []string{"Customer\nSupport agent"}},
		{Step: "design-doc", Question: "reviewers", Unknown: true},
	})
	if err != nil || !slices.Equal(chain, []string{"requirements", "design-doc"}) {
		t.Fatalf("chain %v, %v", chain, err)
	}
	// Answers are now shared facts: Register entries the other documents and the Assistant read.
	var statements []string
	for _, e := range a.Reg.Entries() {
		statements = append(statements, e.Statement)
	}
	if !slices.Contains(statements, "Support agent") {
		t.Errorf("roles weren't recorded: %v", statements)
	}
}

func TestRegisterReachesTheAssistantThenOnlyChanges(t *testing.T) {
	a := pipelineApp(t)
	// Only the Brief's unknowns so far: the Brief line already carries them.
	if got := a.registerContextLocked(); got != "" {
		t.Fatalf("Brief entries sent twice: %q", got)
	}
	if err := a.Answer("requirements", "roles", []string{"Customer"}, false); err != nil {
		t.Fatal(err)
	}
	first := a.registerContextLocked()
	if !strings.HasPrefix(first, "Register (the project's shared facts") || !strings.Contains(first, "[confirmed] Customer") {
		t.Fatalf("first: %q", first)
	}
	if again := a.registerContextLocked(); again != "" {
		t.Fatalf("unchanged Register sent again: %q", again)
	}
	if err := a.Answer("requirements", "roles", []string{"Customer\nSupport agent"}, false); err != nil {
		t.Fatal(err)
	}
	if d := a.registerContextLocked(); !strings.HasPrefix(d, "Register changes:\n+ ") || !strings.Contains(d, "Support agent") {
		t.Fatalf("change: %q", d)
	}
}

func TestConcurrentAnswersDontDuplicateEntries(t *testing.T) {
	a := pipelineApp(t)
	before := len(a.Reg.Entries())
	done := make(chan error, 3)
	for _, an := range []WizardAnswer{
		{Step: "threat-model", Question: "actors", Values: []string{"External attacker"}},
		{Step: "threat-model", Question: "assets", Values: []string{"Customer PII", "Order history"}},
		{Step: "threat-model", Question: "offdiagram", Unknown: true},
	} {
		go func(an WizardAnswer) { done <- a.Answer(an.Step, an.Question, an.Values, an.Unknown) }(an)
	}
	for range 3 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	if n := len(a.Reg.Entries()) - before; n != 3 {
		t.Fatalf("%d new entries, want 3 (one actor, two assets)", n)
	}
	ans, _ := a.pipeStore().LoadAnswers()
	if len(ans["threat-model"]) != 3 {
		t.Fatalf("answers lost: %v", ans["threat-model"])
	}
}
