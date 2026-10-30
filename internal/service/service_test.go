package service

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/config"
)

func cfg() *config.Config {
	return &config.Config{
		Host: config.Host{Name: "ubuntu-a"},
		Forges: config.Forges{
			GitHub: config.GitHub{TokenRef: "switchboard/github", InboxTokenRef: "switchboard/github-inbox"},
			GitLab: config.GitLab{TokenRef: "switchboard/gitlab", InboxTokenRef: "switchboard/gitlab-inbox"},
		},
		Projects: []config.Project{
			{Ref: "gh:o/r", Forge: "gh", Host: "ubuntu-a"},
			{Ref: "gl:nas/a", Forge: "gl", Host: "ubuntu-a"},
			{Ref: "gl:nas/b", Forge: "gl", Host: "ubuntu-a", TokenRef: "switchboard/gitlab-bot", AgentIdentity: "bot"},
			{Ref: "gl:nas/c", Forge: "gl", Host: "elsewhere", TokenRef: "switchboard/gitlab-other", AgentIdentity: "x"},
		},
	}
}

func setup() Setup {
	return Setup{User: "kanda", Home: "/home/kanda", Path: "/home/kanda/.local/bin:/usr/bin", Binary: "/home/kanda/.local/bin/switchboard",
		Config: "/home/kanda/.config/switchboard/config.yaml", Creds: Credentials(cfg())}
}

// Only the agent tokens of this host's projects become credentials; the inbox tokens do not.
func TestCredentialsAreTheAgentTokensOfThisHost(t *testing.T) {
	var refs []string
	for _, c := range Credentials(cfg()) {
		refs = append(refs, c.Ref+"="+c.Name)
	}
	got := strings.Join(refs, " ")
	want := "switchboard/github=switchboard_github switchboard/gitlab=switchboard_gitlab switchboard/gitlab-bot=switchboard_gitlab_bot"
	if got != want {
		t.Fatalf("credentials = %s", got)
	}
}

func TestUnit(t *testing.T) {
	u := setup().Unit()
	for _, want := range []string{
		"User=kanda\n",
		"Environment=HOME=/home/kanda\n",
		"Environment=PATH=/home/kanda/.local/bin:/usr/bin\n",
		"ExecStart=/home/kanda/.local/bin/switchboard agent run --config /home/kanda/.config/switchboard/config.yaml\n",
		"LoadCredentialEncrypted=switchboard_github:/etc/credstore.encrypted/switchboard.switchboard_github\n",
		"LoadCredentialEncrypted=switchboard_gitlab_bot:/etc/credstore.encrypted/switchboard.switchboard_gitlab_bot\n",
		"Restart=on-failure\n",
		"WantedBy=multi-user.target\n",
	} {
		if !strings.Contains(u, want) {
			t.Errorf("the unit lacks %q:\n%s", want, u)
		}
	}
}

func tokens(ref string) (string, error) { return "secret-of-" + ref, nil }

// The tokens go in on standard input; no step shows one.
func TestInstallStepsKeepTokensOffTheCommandLine(t *testing.T) {
	steps, err := setup().InstallSteps("/tmp/unit", tokens)
	if err != nil {
		t.Fatal(err)
	}
	var shown bytes.Buffer
	for _, s := range steps {
		shown.WriteString(s.String() + "\n")
	}
	out := shown.String()
	if strings.Contains(out, "secret-of") {
		t.Fatalf("a token is shown:\n%s", out)
	}
	for _, want := range []string{
		"sudo install -d -m 0700 /etc/credstore.encrypted",
		"sudo systemd-creds encrypt --name=switchboard_github - /etc/credstore.encrypted/switchboard.switchboard_github",
		"sudo install -m 0644 /tmp/unit /etc/systemd/system/switchboard-agent.service",
		"sudo systemctl daemon-reload",
		"sudo systemctl enable switchboard-agent.service",
		"sudo systemctl restart switchboard-agent.service",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the steps lack %q:\n%s", want, out)
		}
	}
	if steps[1].Stdin != "secret-of-switchboard/github" {
		t.Fatalf("the first credential step reads %q", steps[1].Stdin)
	}
}

func TestUninstallSteps(t *testing.T) {
	var shown []string
	for _, s := range setup().UninstallSteps() {
		shown = append(shown, s.String())
	}
	out := strings.Join(shown, "\n")
	for _, want := range []string{
		"sudo systemctl disable --now switchboard-agent.service",
		"sudo rm -f /etc/systemd/system/switchboard-agent.service",
		"sudo rm -f /etc/credstore.encrypted/switchboard.switchboard_gitlab",
		"sudo systemctl daemon-reload",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the steps lack %q:\n%s", want, out)
		}
	}
}

type recorder struct {
	ran  []string
	fail error
}

func (r *recorder) run(s Step) error {
	r.ran = append(r.ran, s.String())
	return r.fail
}

func steps() []Step {
	return []Step{{Args: []string{"systemctl", "daemon-reload"}}, {Args: []string{"systemctl", "restart", "x"}}}
}

func TestConfirmYesRunsEveryStepInOrder(t *testing.T) {
	var out bytes.Buffer
	r := &recorder{}
	if err := Apply(steps(), Confirm{In: strings.NewReader("y\n"), Out: &out, TTY: true}, r.run); err != nil {
		t.Fatal(err)
	}
	if len(r.ran) != 2 || r.ran[0] != "sudo systemctl daemon-reload" {
		t.Fatalf("ran = %v", r.ran)
	}
	// Every step is listed before the question.
	if i, j := strings.Index(out.String(), "sudo systemctl restart x"), strings.Index(out.String(), "[y/N]"); i < 0 || j < i {
		t.Fatalf("output:\n%s", out.String())
	}
}

func TestDecliningRunsNothing(t *testing.T) {
	for _, answer := range []string{"n\n", "\n", ""} {
		r := &recorder{}
		err := Apply(steps(), Confirm{In: strings.NewReader(answer), Out: &bytes.Buffer{}, TTY: true}, r.run)
		if !errors.Is(err, ErrDeclined) || len(r.ran) != 0 {
			t.Fatalf("answer %q: err=%v ran=%v", answer, err, r.ran)
		}
	}
}

func TestDryRunOnlyShows(t *testing.T) {
	var out bytes.Buffer
	r := &recorder{}
	if err := Apply(steps(), Confirm{DryRun: true, Out: &out}, r.run); err != nil || len(r.ran) != 0 {
		t.Fatalf("err=%v ran=%v", err, r.ran)
	}
	if !strings.Contains(out.String(), "sudo systemctl daemon-reload") {
		t.Fatalf("output:\n%s", out.String())
	}
}

func TestWithoutATerminalYesIsNeeded(t *testing.T) {
	r := &recorder{}
	err := Apply(steps(), Confirm{In: strings.NewReader("y\n"), Out: &bytes.Buffer{}}, r.run)
	if err == nil || !strings.Contains(err.Error(), "--yes") || len(r.ran) != 0 {
		t.Fatalf("err=%v ran=%v", err, r.ran)
	}
	if err := Apply(steps(), Confirm{Yes: true, Out: &bytes.Buffer{}}, r.run); err != nil || len(r.ran) != 2 {
		t.Fatalf("--yes: err=%v ran=%v", err, r.ran)
	}
}

// A failing step stops the rest.
func TestAFailingStepStops(t *testing.T) {
	r := &recorder{fail: errors.New("exit 1")}
	err := Apply(steps(), Confirm{Yes: true, Out: &bytes.Buffer{}}, r.run)
	if err == nil || len(r.ran) != 1 {
		t.Fatalf("err=%v ran=%v", err, r.ran)
	}
}

// status reads the unit and asks systemctl, as the user: it never goes through sudo.
func TestStatusNeedsNoSudo(t *testing.T) {
	var asked []string
	systemctl := func(args ...string) string {
		asked = append(asked, strings.Join(args, " "))
		if args[0] == "is-enabled" {
			return "enabled"
		}
		return "active"
	}
	read := func(path string) (string, error) {
		if path != UnitPath {
			t.Fatalf("read %s", path)
		}
		return setup().Unit(), nil
	}
	var out bytes.Buffer
	Status(&out, read, systemctl)
	s := out.String()
	for _, want := range []string{"unit: " + UnitPath, "credentials: switchboard_github, switchboard_gitlab, switchboard_gitlab_bot", "enabled: enabled", "active: active"} {
		if !strings.Contains(s, want) {
			t.Errorf("status lacks %q:\n%s", want, s)
		}
	}
	if strings.Join(asked, "; ") != "is-enabled "+Name+"; is-active "+Name {
		t.Fatalf("systemctl was asked: %v", asked)
	}
}

func TestStatusWithoutAUnit(t *testing.T) {
	var out bytes.Buffer
	Status(&out, func(string) (string, error) { return "", errors.New("no such file") }, func(...string) string { return "disabled" })
	if !strings.Contains(out.String(), "unit: not installed") {
		t.Fatalf("status:\n%s", out.String())
	}
}
