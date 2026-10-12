package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/vinaayakh/designer/internal/register"
)

// Entries proposed in chat (wayfinder/doc-pipeline ticket 003 §4): the Assistant calls
// propose_entries, the user reviews each on a card, and only accepted ones reach the Register.
// Chat never changes the audit trail by itself.

const proposedFile = "proposed-entries.json"

// ProposedEntry waits for the user to accept or reject it.
type ProposedEntry struct {
	ID       string       `json:"id"` // e1, e2, …
	Entry    register.New `json:"entry"`
	Replaces string       `json:"replaces,omitempty"` // a live entry this one supersedes when accepted
	Created  time.Time    `json:"created"`
}

type proposedState struct {
	Next    int              `json:"next"`
	Entries []*ProposedEntry `json:"entries"`
}

func (a *App) proposedLocked() proposedState {
	var st proposedState
	_ = a.P.ReadState(proposedFile, &st)
	return st
}

// ProposedEntries lists the entries waiting for review.
func (a *App) ProposedEntries() []*ProposedEntry {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.proposedLocked().Entries
}

func (a *App) proposeEntriesLocked(raw json.RawMessage) (string, error) {
	var in struct {
		Entries []struct {
			Type      register.Type     `json:"type"`
			Statement string            `json:"statement"`
			Fields    map[string]string `json:"fields"`
			Links     []string          `json:"links"`
			Replaces  string            `json:"replaces"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", err
	}
	if len(in.Entries) == 0 {
		return "", errors.New("propose at least one entry")
	}
	if len(in.Entries) > 10 {
		return "", errors.New("propose at most 10 entries at a time")
	}
	text := ""
	if a.turn != nil && a.turn.spec != nil {
		text = a.turn.spec.text
	}
	live := map[string]string{}
	for _, e := range a.Reg.Entries() {
		if e.Live() {
			live[strings.ToLower(e.Statement)] = e.ID
		}
	}
	st := a.proposedLocked()
	var news []*ProposedEntry
	var problems []string
	for i, e := range in.Entries {
		statement := strings.TrimSpace(e.Statement)
		var links []string
		for _, al := range e.Links {
			id, ok := a.sess.Aliases.ID(al)
			if !ok {
				problems = append(problems, fmt.Sprintf("entry %d links %s, which isn't on the diagram", i+1, al))
				continue
			}
			links = append(links, id)
		}
		n := register.New{Type: e.Type, Statement: statement, Fields: e.Fields, Links: links,
			Source: register.Source{Kind: "chat", Text: truncate(text, 2000)}, Status: register.Confirmed}
		if err := register.Validate(n); err != nil {
			problems = append(problems, fmt.Sprintf("entry %d: %v", i+1, err))
		}
		if e.Replaces != "" {
			if old, ok := a.Reg.Get(e.Replaces); !ok || !old.Live() {
				problems = append(problems, fmt.Sprintf("entry %d replaces %s, which isn't a live Register entry", i+1, e.Replaces))
			}
		}
		if id, dup := live[strings.ToLower(statement)]; dup {
			problems = append(problems, fmt.Sprintf("entry %d is already in the Register as %s", i+1, id))
		}
		if slices.ContainsFunc(st.Entries, func(p *ProposedEntry) bool { return strings.EqualFold(p.Entry.Statement, statement) }) {
			problems = append(problems, fmt.Sprintf("entry %d is already waiting for review", i+1))
		}
		news = append(news, &ProposedEntry{Entry: n, Replaces: e.Replaces})
	}
	if len(problems) > 0 {
		return "", errors.New("nothing was proposed; fix these and call propose_entries again:\n- " + strings.Join(problems, "\n- "))
	}
	for _, pe := range news {
		st.Next++
		pe.ID, pe.Created = fmt.Sprintf("e%d", st.Next), time.Now().UTC()
		st.Entries = append(st.Entries, pe)
	}
	if err := a.P.WriteState(proposedFile, st); err != nil {
		return "", err
	}
	a.emit("register.proposed", map[string]any{"count": len(st.Entries)})
	return fmt.Sprintf("%d entries are waiting for the user's review. They aren't in the Register until accepted. Tell the user in one sentence what you proposed.", len(news)), nil
}

// DecideProposedEntry accepts (optionally with edited text) or rejects one proposed entry.
func (a *App) DecideProposedEntry(id string, accept bool, statement string) (*register.Entry, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	st := a.proposedLocked()
	i := slices.IndexFunc(st.Entries, func(p *ProposedEntry) bool { return p.ID == id })
	if i < 0 {
		return nil, fmt.Errorf("no proposed entry %s", id)
	}
	pe := st.Entries[i]
	var created *register.Entry
	if accept {
		n := pe.Entry
		if s := strings.TrimSpace(statement); s != "" {
			n.Statement = s
		}
		n.Status = register.Confirmed
		var e register.Entry
		var err error
		if old, ok := a.Reg.Get(pe.Replaces); ok && old.Live() {
			e, err = a.Reg.Supersede(register.ActorUser, old.Step, "", old.ID, n, "accepted from chat")
		} else {
			e, err = a.Reg.Create(register.ActorUser, "", "", n, "accepted from chat")
		}
		if err != nil {
			return nil, err
		}
		created = &e
	}
	st.Entries = slices.Delete(st.Entries, i, i+1)
	if err := a.P.WriteState(proposedFile, st); err != nil {
		return nil, err
	}
	what := "rejected"
	if created != nil {
		what = "accepted as " + created.ID
	}
	a.queueFeedbackLocked(fmt.Sprintf(`{"proposed_entry":%q,"status":%q}`, pe.Entry.Statement, what))
	a.emit("register", nil)
	return created, nil
}
