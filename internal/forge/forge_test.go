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

func TestChangeRequestNumber(t *testing.T) {
	for url, want := range map[string]int{
		"https://github.com/o/r/pull/48":                    48,
		"https://gl.example/nas/tools/x/-/merge_requests/7": 7,
		"https://github.com/o/r/issues/3":                   0,
		"":                                                  0,
	} {
		if got := ChangeRequestNumber(url); got != want {
			t.Errorf("ChangeRequestNumber(%q) = %d, want %d", url, got, want)
		}
	}
}

func TestCheckFailed(t *testing.T) {
	for _, c := range []struct {
		check Check
		want  bool
	}{
		{Check{Done: true, Conclusion: Failure}, true},
		{Check{Done: true, Conclusion: TimedOut}, true},
		{Check{Done: true, Conclusion: Cancelled}, false},
		{Check{Done: true, Conclusion: Skipped}, false},
		{Check{Done: true, Conclusion: Neutral}, false},
		{Check{Done: false, Conclusion: Failure}, false},
	} {
		if got := c.check.Failed(); got != c.want {
			t.Errorf("%+v.Failed() = %v, want %v", c.check, got, c.want)
		}
	}
}
