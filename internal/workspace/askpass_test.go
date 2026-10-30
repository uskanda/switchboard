package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// git asks GIT_ASKPASS for the user name, then the password. GitHub takes x-access-token with a token as the
// password; GitLab takes oauth2.
func TestAskpassAnswersTheForgesUserName(t *testing.T) {
	for _, c := range []struct{ user, want string }{{"", "x-access-token"}, {"oauth2", "oauth2"}} {
		script := filepath.Join(t.TempDir(), "askpass")
		if err := os.WriteFile(script, []byte(askpassScript(c.user)), 0o700); err != nil {
			t.Fatal(err)
		}
		ask := func(prompt string) string {
			cmd := exec.Command(script, prompt)
			cmd.Env = append(os.Environ(), "SWITCHBOARD_GIT_TOKEN=tok")
			out, err := cmd.Output()
			if err != nil {
				t.Fatal(err)
			}
			return strings.TrimSpace(string(out))
		}
		if got := ask("Username for 'https://gl.test': "); got != c.want {
			t.Errorf("user %q: username = %q", c.user, got)
		}
		if got := ask("Password for 'https://oauth2@gl.test': "); got != "tok" {
			t.Errorf("user %q: password = %q", c.user, got)
		}
	}
}

func TestGitLabCIIsAProtectedPath(t *testing.T) {
	found := false
	for _, p := range protected {
		found = found || p == ".gitlab-ci.yml"
	}
	if !found {
		t.Fatalf("protected = %v", protected)
	}
}
