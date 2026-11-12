package github

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/uskanda/switchboard/internal/forge"
)

// GitHub has no public API that attaches a file to a comment; the GitHub CLI does, from v2.99.0, through the upload
// the web page uses (ADR-051, V47). This file is the only place switchboard runs gh.

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
	id, err := c.ghAttach(ctx, ref, body, files)
	if err != nil {
		return nil, err
	}
	if id == 0 {
		return &forge.Comment{Body: body}, nil // posted; gh did not say which comment
	}
	got, err := c.readComment(ctx, ref, id)
	if err != nil {
		return nil, fmt.Errorf("read the attached comment: %w", err)
	}
	_, last := forge.SplitLastLine(body)
	if strings.HasSuffix(strings.TrimRight(got.Body, " \t\r\n"), last) {
		return got, nil
	}
	fixed := strings.TrimRight(strings.Replace(got.Body, last, "", 1), " \t\r\n") + "\n\n" + last + "\n"
	if err := c.EditComment(ctx, ref, id, fixed); err != nil {
		return nil, fmt.Errorf("put the marker last: %w", err)
	}
	got.Body = fixed
	return got, nil
}

// attachedImage is an image gh put after the body: ![name](https://github.com/user-attachments/assets/<uuid>).
var attachedImage = regexp.MustCompile(`!\[([^\]\n]*)\]\((https://github\.com/user-attachments/assets/[0-9a-fA-F-]+)\)`)

// AttachPlaced implements forge.PlacedAttacher (ADR-071): it posts body with gh issue comment --attach, as
// AttachComment does, then edits the comment so that the image of each file, which gh put after the body, takes the
// place of the file's mark. The images gh put are matched to the files in order, and each by its name, the file's
// name without its extension. An edit that fails is tried once more; when the images cannot be put in place, the
// comment is deleted.
func (c *Client) AttachPlaced(ctx context.Context, ref forge.IssueRef, body string, images []forge.PlacedImage) (*forge.Comment, error) {
	files := make([]string, len(images))
	for i, im := range images {
		files[i] = im.Path
	}
	id, err := c.ghAttach(ctx, ref, body, files)
	if err != nil {
		return nil, &forge.PlacedError{Err: err}
	}
	if id == 0 {
		return nil, &forge.PlacedError{Err: errors.New("gh did not say which comment it posted"), Left: true}
	}
	got, err := c.readComment(ctx, ref, id)
	var placed string
	if err == nil {
		placed, err = placeImages(body, got.Body, images)
	}
	if err == nil {
		if err = c.EditComment(ctx, ref, id, placed); err != nil {
			err = c.EditComment(ctx, ref, id, placed)
		}
	}
	if err != nil {
		left := c.DeleteComment(ctx, ref, id) != nil
		return nil, &forge.PlacedError{Err: fmt.Errorf("put the images in place: %w", err), Left: left}
	}
	got.Body = placed
	return got, nil
}

// placeImages returns body with the mark of each image replaced by the image gh put after the body in posted.
func placeImages(body, posted string, images []forge.PlacedImage) (string, error) {
	found := attachedImage.FindAllStringSubmatch(posted, -1)
	if len(found) != len(images) {
		return "", fmt.Errorf("gh attached %d images, not %d", len(found), len(images))
	}
	for i, im := range images {
		name := filepath.Base(im.Path)
		name = strings.TrimSuffix(name, filepath.Ext(name))
		if found[i][1] != name {
			return "", fmt.Errorf("the image %d is %q, not %q", i+1, found[i][1], name)
		}
		if !strings.Contains(body, im.Mark) {
			return "", fmt.Errorf("the body has no mark %q", im.Mark)
		}
		body = strings.Replace(body, im.Mark, found[i][0], 1)
	}
	return body, nil
}

// ghAttach runs gh issue comment --attach and returns the ID of the comment it posted; 0 when gh did not say.
func (c *Client) ghAttach(ctx context.Context, ref forge.IssueRef, body string, files []string) (int64, error) {
	cfg, err := os.MkdirTemp("", "switchboard-gh-*")
	if err != nil {
		return 0, err
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
		return 0, fmt.Errorf("gh issue comment --attach: %w: %s", err, msg)
	}
	m := commentURL.FindStringSubmatch(out.String())
	if m == nil {
		return 0, nil
	}
	id, _ := strconv.ParseInt(m[1], 10, 64)
	return id, nil
}

// readComment reads the issue comment id.
func (c *Client) readComment(ctx context.Context, ref forge.IssueRef, id int64) (*forge.Comment, error) {
	var gc ghComment
	if _, err := c.do(ctx, http.MethodGet, c.url(fmt.Sprintf("/repos/%s/%s/issues/comments/%d", ref.Owner, ref.Repo, id), nil), nil, &gc, nil); err != nil {
		return nil, err
	}
	return &forge.Comment{ID: gc.ID, Body: gc.Body, Author: gc.User.user(), CreatedAt: gc.CreatedAt}, nil
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
