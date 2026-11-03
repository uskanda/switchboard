package github

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/uskanda/switchboard/internal/forge"
)

// GitHub has no public API that attaches a file to a comment; the GitHub CLI does, from v2.99.0, through the upload
// the web page uses (ADR-050, V46). This file is the only place switchboard runs gh.

// GHMinVersion is the first gh version with --attach.
const GHMinVersion = "2.99.0"

// WithGH sets the gh binary AttachComment runs; "gh" from PATH by default.
func WithGH(binary string) Option { return func(c *Client) { c.gh = binary } }

func (c *Client) ghBinary() string {
	if c.gh == "" {
		return "gh"
	}
	return c.gh
}

var commentURL = regexp.MustCompile(`#issuecomment-(\d+)`)

// ghEnv is the whole environment gh runs with: PATH, and home, configuration and state directories in dir, an empty
// directory removed afterwards, so that gh neither reads the host user's account nor writes anywhere else.
func ghEnv(dir string) []string {
	return []string{
		"PATH=" + os.Getenv("PATH"), "HOME=" + dir, "GH_CONFIG_DIR=" + dir,
		"XDG_CONFIG_HOME=" + dir, "XDG_STATE_HOME=" + dir, "XDG_DATA_HOME=" + dir, "XDG_CACHE_HOME=" + dir,
		"GH_PROMPT_DISABLED=1", "GH_NO_UPDATE_NOTIFIER=1", "NO_COLOR=1",
	}
}

// AttachComment implements forge.Attacher with gh issue comment --attach. gh gets the client's token in GH_TOKEN, an
// empty configuration directory, and no other variable of the environment but PATH, so that it reads no account of
// the host's user. The body goes in on standard input. When gh puts the attachments after the body, the comment is
// edited so that the body's last line, the marker, ends it again.
func (c *Client) AttachComment(ctx context.Context, ref forge.IssueRef, body string, files []string) (*forge.Comment, error) {
	cfg, err := os.MkdirTemp("", "switchboard-gh-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(cfg)
	args := []string{"issue", "comment", strconv.Itoa(ref.Number), "-R", ref.Owner + "/" + ref.Repo, "--body-file", "-"}
	for _, f := range files {
		args = append(args, "--attach", f)
	}
	cmd := exec.CommandContext(ctx, c.ghBinary(), args...)
	cmd.Env = append(ghEnv(cfg), "GH_TOKEN="+c.token)
	cmd.Stdin = strings.NewReader(body)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String() + "\n" + out.String())
		if len(msg) > 500 {
			msg = msg[len(msg)-500:]
		}
		return nil, fmt.Errorf("gh issue comment --attach: %w: %s", err, msg)
	}
	m := commentURL.FindStringSubmatch(out.String())
	if m == nil {
		return &forge.Comment{Body: body}, nil // posted; gh did not say which comment
	}
	id, _ := strconv.ParseInt(m[1], 10, 64)
	var gc ghComment
	if _, err := c.do(ctx, http.MethodGet, c.url(fmt.Sprintf("/repos/%s/%s/issues/comments/%d", ref.Owner, ref.Repo, id), nil), nil, &gc, nil); err != nil {
		return nil, fmt.Errorf("read the attached comment: %w", err)
	}
	got := &forge.Comment{ID: gc.ID, Body: gc.Body, Author: gc.User.user(), CreatedAt: gc.CreatedAt}
	_, last := forge.SplitLastLine(body)
	if strings.HasSuffix(strings.TrimRight(gc.Body, " \t\r\n"), last) {
		return got, nil
	}
	fixed := strings.TrimRight(strings.Replace(gc.Body, last, "", 1), " \t\r\n") + "\n\n" + last + "\n"
	if err := c.EditComment(ctx, ref, id, fixed); err != nil {
		return nil, fmt.Errorf("put the marker last: %w", err)
	}
	got.Body = fixed
	return got, nil
}

var ghVersion = regexp.MustCompile(`(?m)^gh version (\d+\.\d+\.\d+)`)

// GHVersion runs gh --version and returns its version, such as "2.99.0".
func GHVersion(ctx context.Context, binary string) (string, error) {
	if binary == "" {
		binary = "gh"
	}
	dir, err := os.MkdirTemp("", "switchboard-gh-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	cmd := exec.CommandContext(ctx, binary, "--version")
	cmd.Env = ghEnv(dir)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	if v := parseGHVersion(string(out)); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("gh --version printed no version: %q", strings.TrimSpace(string(out)))
}

func parseGHVersion(out string) string {
	if m := ghVersion.FindStringSubmatch(out); m != nil {
		return m[1]
	}
	return ""
}

// GHAttaches reports whether gh of version v has --attach.
func GHAttaches(v string) bool {
	have, want := strings.Split(v, "."), strings.Split(GHMinVersion, ".")
	if len(have) != 3 {
		return false
	}
	for i := range want {
		h, err := strconv.Atoi(have[i])
		if err != nil {
			return false
		}
		w, _ := strconv.Atoi(want[i])
		if h != w {
			return h > w
		}
	}
	return true
}
