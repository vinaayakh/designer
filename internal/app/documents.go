package app

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/vinaayakh/designer/internal/pipeline"
	"github.com/vinaayakh/designer/internal/scene"
)

// The Documents panel lists every document with a Create button. Creating one
// records the wizard's answers in one go and writes it, after any document it
// builds on that isn't written yet: those are queued and written first, one
// Assistant task each. Answers become Register entries, the shared facts every
// document and the Assistant use.

// WizardAnswer is one answer from the Create wizard.
type WizardAnswer struct {
	Step     string   `json:"step"`
	Question string   `json:"question"`
	Values   []string `json:"values"`
	Unknown  bool     `json:"unknown"`
}

// DocumentChain lists what Create writes, in order: the documents stepID
// requires that aren't written yet (theirs first), then stepID itself.
func (a *App) DocumentChain(stepID string) ([]string, error) {
	s := pipeline.Get(stepID)
	if s == nil {
		return nil, fmt.Errorf("no document %q", stepID)
	}
	now, err := a.pipelineNow()
	if err != nil {
		return nil, err
	}
	var out []string
	var visit func(s *pipeline.Step)
	visit = func(s *pipeline.Step) {
		for _, r := range s.Requires {
			if now.Latest[r] == nil && !slices.Contains(out, r) {
				visit(pipeline.Get(r))
			}
		}
		if !slices.Contains(out, s.ID) {
			out = append(out, s.ID)
		}
	}
	visit(s)
	return out, nil
}

// RecordAnswers checks every answer, then records them all (Edit answers in the Documents panel).
func (a *App) RecordAnswers(stepID string, answers []WizardAnswer) error {
	if problems := checkAnswers([]string{stepID}, answers); len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	for _, an := range answers {
		if err := a.Answer(an.Step, an.Question, an.Values, an.Unknown); err != nil {
			return err
		}
	}
	return nil
}

// checkAnswers lists every problem with a set of answers for the given documents.
func checkAnswers(steps []string, answers []WizardAnswer) []string {
	var problems []string
	for _, an := range answers {
		s := pipeline.Get(an.Step)
		if s == nil || !slices.Contains(steps, an.Step) {
			problems = append(problems, fmt.Sprintf("%q isn't part of writing this document", an.Step))
			continue
		}
		q := s.Question(an.Question)
		switch {
		case q == nil:
			problems = append(problems, fmt.Sprintf("%s has no question %q", s.Title, an.Question))
		case an.Unknown && q.Blocking:
			problems = append(problems, fmt.Sprintf("%q is needed to write the %s", q.Label, s.Title))
		case !an.Unknown:
			if _, err := q.Clean(an.Values); err != nil {
				problems = append(problems, fmt.Sprintf("%s: %v", q.Label, err))
			}
		}
	}
	return problems
}

// prepareDocument checks every answer before recording any (all problems at
// once), records them, and returns the chain to write. A required question
// still unanswered anywhere in the chain stops it.
func (a *App) prepareDocument(stepID string, answers []WizardAnswer) ([]string, error) {
	chain, err := a.DocumentChain(stepID)
	if err != nil {
		return nil, err
	}
	problems := checkAnswers(chain, answers)
	if len(problems) > 0 {
		return nil, errors.New(strings.Join(problems, "; "))
	}
	for _, an := range answers {
		if err := a.Answer(an.Step, an.Question, an.Values, an.Unknown); err != nil {
			return nil, err
		}
	}
	now, err := a.pipelineNow()
	if err != nil {
		return nil, err
	}
	for _, id := range chain {
		if st := pipeline.StatusOf(pipeline.Get(id), now); len(st.Blocking) > 0 {
			problems = append(problems, fmt.Sprintf("the %s needs %s", pipeline.Get(id).Title, strings.Join(st.Blocking, " and ")))
		}
	}
	if len(problems) > 0 {
		return nil, errors.New(strings.Join(problems, "; "))
	}
	return chain, nil
}

// CreateDocument records the answers and starts writing: the first document
// now, the rest when each one before it is done.
func (a *App) CreateDocument(stepID string, answers []WizardAnswer) error {
	a.mu.Lock()
	busy := a.turn != nil || len(a.docQueue) > 0
	a.mu.Unlock()
	if busy {
		return ErrBusy
	}
	chain, err := a.prepareDocument(stepID, answers)
	if err != nil {
		return err
	}
	a.mu.Lock()
	a.docQueue = chain[1:]
	a.mu.Unlock()
	if err := a.RunStep(chain[0]); err != nil {
		a.mu.Lock()
		a.docQueue = nil
		a.mu.Unlock()
		return err
	}
	a.emit("pipeline", nil)
	return nil
}

// nextDocumentLocked runs after a document task ends: the next queued
// document starts if this one was written; otherwise the queue is dropped.
func (a *App) nextDocumentLocked(written bool) {
	if len(a.docQueue) == 0 {
		return
	}
	if !written {
		a.docQueue = nil
		a.emit("pipeline", nil)
		return
	}
	next := a.docQueue[0]
	a.docQueue = a.docQueue[1:]
	go func() {
		if err := a.RunStep(next); err != nil {
			slog.Error("queued document didn't start", "step", next, "err", err)
			a.mu.Lock()
			a.docQueue = nil
			a.mu.Unlock()
			a.emit("assistant.done", map[string]any{"kind": "step", "failed": fmt.Sprintf("The %s couldn't start: %v", pipeline.Get(next).Title, err)})
		}
		a.emit("pipeline", nil)
	}()
}

// registerSnapshot is the Register as the Assistant sees it while refining the
// diagram: one line per live entry, so later messages carry only changes.
func (a *App) registerSnapshot() scene.Snapshot {
	s := scene.Snapshot{}
	for _, e := range a.Reg.Entries() {
		// Entries made from the Brief repeat what the Brief line already says.
		if !e.Live() || e.Source.Kind == "brief" {
			continue
		}
		st := e.Statement
		if len(st) > 200 {
			st = st[:199] + "…"
		}
		line := fmt.Sprintf("%s [%s] %s", e.ID, e.Status, strings.Join(strings.Fields(st), " "))
		if len(e.Links) > 0 {
			als := make([]string, len(e.Links))
			for i, id := range e.Links {
				if als[i] = a.sess.Aliases.Peek(id); als[i] == "" {
					als[i] = id
				}
			}
			line += " (on " + strings.Join(als, ", ") + ")"
		}
		s[e.ID] = line
	}
	return s
}

// registerContextLocked is the Register part of a chat message: everything
// the first time, then only what changed.
func (a *App) registerContextLocked() string {
	if a.Reg == nil {
		return ""
	}
	snap := a.registerSnapshot()
	defer func() { a.sess.SentRegister = snap }()
	if len(a.sess.SentRegister) == 0 {
		if len(snap) == 0 {
			return ""
		}
		lines := make([]string, 0, len(snap))
		for _, l := range snap {
			lines = append(lines, l)
		}
		slices.Sort(lines)
		return "Register (the project's shared facts; confirmed entries are agreed, open ones aren't yet):\n" + strings.Join(lines, "\n")
	}
	if d := scene.Diff(a.sess.SentRegister, snap, nil); d != "" {
		return "Register changes:\n" + d
	}
	return ""
}
