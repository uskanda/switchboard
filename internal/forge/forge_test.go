package forge

import "testing"

func TestIssueRefStringCarriesTheForge(t *testing.T) {
	for _, c := range []struct {
		ref  IssueRef
		want string
	}{
		{IssueRef{Forge: GitHub, Owner: "uskanda", Repo: "sandbox", Number: 1}, "gh:uskanda/sandbox#1"},
		{IssueRef{Forge: GitLab, Owner: "nas/tools", Repo: "sandbox", Number: 3}, "gl:nas/tools/sandbox#3"},
		// A ref without a forge is a GitHub one: Phase 1 code and its records never set the field.
		{IssueRef{Owner: "uskanda", Repo: "sandbox", Number: 2}, "gh:uskanda/sandbox#2"},
	} {
		if got := c.ref.String(); got != c.want {
			t.Errorf("%+v.String() = %q, want %q", c.ref, got, c.want)
		}
	}
}

func TestIssueRefKindAndProject(t *testing.T) {
	r := IssueRef{Forge: GitLab, Owner: "nas/tools", Repo: "sandbox", Number: 3}
	if r.Kind() != GitLab || r.Project() != "nas/tools/sandbox" {
		t.Fatalf("Kind() = %q, Project() = %q", r.Kind(), r.Project())
	}
	if (IssueRef{Owner: "o", Repo: "r"}).Kind() != GitHub {
		t.Fatal("a ref without a forge is not GitHub")
	}
}
