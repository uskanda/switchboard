package inbox

import (
	"reflect"
	"testing"

	"github.com/uskanda/switchboard/internal/config"
)

// Without search_scope, the GitHub search covers the configured GitHub projects, never the whole of GitHub.
func TestGitHubScopesWithoutSearchScope(t *testing.T) {
	cfg := &config.Config{}
	for _, ref := range []string{"gh:o/a", "gh:o/b", "gl:nas/c", "gh:o/d", "gh:o/e", "gh:o/f", "gh:o/a"} {
		p, err := config.ParseRef(ref)
		if err != nil {
			t.Fatal(err)
		}
		cfg.Projects = append(cfg.Projects, p)
	}
	got := (&Inbox{Config: cfg}).githubScopes()
	want := []string{"repo:o/a repo:o/b repo:o/d repo:o/e", "repo:o/f"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scopes = %q, want %q", got, want)
	}
	cfg.Forges.GitHub.SearchScope = "user:o"
	if got := (&Inbox{Config: cfg}).githubScopes(); !reflect.DeepEqual(got, []string{"user:o"}) {
		t.Fatalf("scopes = %q", got)
	}
}

func TestProjectMatchesWithoutRegardToCase(t *testing.T) {
	p, _ := config.ParseRef("gh:Alice/Sandbox")
	b := &Inbox{Config: &config.Config{Projects: []config.Project{p}}}
	if _, ok := b.project("gh", "alice", "sandbox"); !ok {
		t.Fatal("the project was not found in another case")
	}
}
