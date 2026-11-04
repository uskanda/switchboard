package inbox

import (
	"reflect"
	"testing"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
)

// Each forge is searched over its own configured projects, each once; GitHub's search_scope goes along as the query.
func TestScopeOfEachForge(t *testing.T) {
	cfg := &config.Config{}
	for _, ref := range []string{"gh:o/a", "gh:o/b", "gl:nas/c", "gh:o/a"} {
		p, err := config.ParseRef(ref)
		if err != nil {
			t.Fatal(err)
		}
		cfg.Projects = append(cfg.Projects, p)
	}
	b := &Inbox{Config: cfg}
	if got, want := b.scopeOf(forge.GitHub), forge.Projects("o/a", "o/b"); !reflect.DeepEqual(got, want) {
		t.Fatalf("GitHub scope = %+v, want %+v", got, want)
	}
	if got, want := b.scopeOf(forge.GitLab), forge.Projects("nas/c"); !reflect.DeepEqual(got, want) {
		t.Fatalf("GitLab scope = %+v, want %+v", got, want)
	}
	cfg.Forges.GitHub.SearchScope = "user:o"
	if got := b.scopeOf(forge.GitHub); got.Query != "user:o" {
		t.Fatalf("GitHub scope = %+v", got)
	}
	if got := b.scopeOf(forge.GitLab); got.Query != "" {
		t.Fatalf("GitLab scope = %+v", got)
	}
}

func TestProjectMatchesWithoutRegardToCase(t *testing.T) {
	p, _ := config.ParseRef("gh:Alice/Sandbox")
	b := &Inbox{Config: &config.Config{Projects: []config.Project{p}}}
	if _, ok := b.project("gh", "alice", "sandbox"); !ok {
		t.Fatal("the project was not found in another case")
	}
}
