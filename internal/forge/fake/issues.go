package fake

import (
	"context"

	"github.com/uskanda/switchboard/internal/forge"
)

var _ forge.Issues = (*Forge)(nil)

// CreateIssue implements forge.Issues. The issue is numbered after the highest one of the repository and authored by
// the user named by PostAs ("agent" by default), who is also recorded as adding each label.
func (f *Forge) CreateIssue(_ context.Context, owner, name, title, body string, labels []string) (*forge.IssueSummary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.repos[owner+"/"+name]
	if !ok {
		return nil, forge.ErrNotFound
	}
	n := 1
	for k := range r.issues {
		n = max(n, k+1)
	}
	as := f.PostAs
	if as == "" {
		as = "agent"
	}
	by := f.users[as]
	ref := forge.IssueRef{Forge: f.kind, Owner: owner, Repo: name, Number: n}
	now := f.now()
	is := &issue{
		data: forge.Issue{
			IssueSummary: forge.IssueSummary{Ref: ref, Title: title, State: "open", URL: f.issueURL(ref), UpdatedAt: now,
				Labels: append([]string(nil), labels...)},
			Body: body, Author: by, CreatedAt: now,
		},
		labeled: map[string]forge.User{},
	}
	for _, l := range labels {
		is.labeled[l] = by
	}
	r.issues[n] = is
	s := f.summary(is)
	return &s, nil
}
