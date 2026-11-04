package doctor

import (
	"context"
	"errors"
	"testing"

	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/forge/fake"
)

// prober is the fake forge with a token that tells its scopes, or does not (known false), and reads that fail.
type prober struct {
	*fake.Forge
	scopes  *forge.TokenScopes
	failing string // the read that fails
}

func (p prober) TokenScopes(context.Context) (*forge.TokenScopes, error) { return p.scopes, nil }

func (p prober) ProbeReads(_ context.Context, _, _, _ string) []forge.Probe {
	var out []forge.Probe
	for _, what := range []string{"issues", "pull requests", "contents"} {
		var err error
		if what == p.failing {
			err = forge.ErrForbidden
		}
		out = append(out, forge.Probe{What: what, Err: err})
	}
	return out
}

func withProber(w *world, p prober) {
	p.Forge = w.f
	w.d.Forge = func(_, _ string) forge.Forge { return p }
}

func TestClassicTokenScopes(t *testing.T) {
	for _, c := range []struct {
		scopes      []string
		level, text string
		failures    int
	}{
		{[]string{"public_repo"}, "ok", "has the scopes [public_repo]", 0},
		{[]string{"repo", "admin:org"}, "warn", "repo, admin:org reach beyond the projects", 0},
		{[]string{"read:user"}, "fail", "needs repo (private repositories) or public_repo", 2},
	} {
		w := newWorld(t)
		withProber(w, prober{scopes: &forge.TokenScopes{Known: true, Scopes: c.scopes}})
		if n := w.run(t); n != c.failures {
			t.Errorf("%v: %d failures:\n%s", c.scopes, n, w.out.String())
		}
		w.expect(t, c.level, c.text)
	}
}

func TestGitLabTokenNeedsTheAPIScope(t *testing.T) {
	for _, c := range []struct {
		scopes      []string
		level, text string
	}{
		{[]string{"read_api"}, "fail", "has the scopes [read_api]; switchboard needs the api scope"},
		{[]string{"api", "sudo"}, "warn", "sudo reach beyond the projects"},
		{[]string{"api"}, "ok", "has the scopes [api]"},
	} {
		w := newWorld(t)
		g, _ := withGitLab(t, w, "private")
		gp := prober{Forge: g, scopes: &forge.TokenScopes{Known: true, Scopes: c.scopes}}
		inner := w.d.Forge
		w.d.Forge = func(kind, token string) forge.Forge {
			if kind == forge.GitLab {
				return gp
			}
			return inner(kind, token)
		}
		w.run(t)
		w.expect(t, c.level, c.text)
	}
}

// A token whose scopes cannot be read is tried like a fine-grained one.
func TestUnreadableScopesAreAWarning(t *testing.T) {
	w := newWorld(t)
	p := failingScopes{prober{Forge: w.f}}
	w.d.Forge = func(_, _ string) forge.Forge { return p }
	if n := w.run(t); n != 0 {
		t.Fatalf("%d failures:\n%s", n, w.out.String())
	}
	w.expect(t, "warn", "its scopes could not be read")
	w.expect(t, "ok", "gh:o/r: the agent token can read the contents")
}

type failingScopes struct{ prober }

func (failingScopes) TokenScopes(context.Context) (*forge.TokenScopes, error) {
	return nil, errors.New("503")
}

// A fine-grained token tells nothing of its permissions: its reads are tried, and its writes are named as unchecked.
func TestFineGrainedTokenReadsAreTried(t *testing.T) {
	w := newWorld(t)
	withProber(w, prober{scopes: &forge.TokenScopes{}, failing: "contents"})
	if n := w.run(t); n != 1 {
		t.Fatalf("%d failures:\n%s", n, w.out.String())
	}
	w.expect(t, "ok", "fine-grained token")
	w.expect(t, "ok", "gh:o/r: the agent token can read the issues")
	w.expect(t, "fail", "gh:o/r: the agent token cannot read the contents")
}
