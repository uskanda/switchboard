//go:build realgh

package github

// This test runs the real gh CLI against a real issue (V43): it posts a comment with an image. It runs only with the
// realgh build tag, and needs the agent's token and a sandbox issue:
//
//	SWITCHBOARD_GH_TOKEN=... SWITCHBOARD_GH_ISSUE=owner/repo#1 go test -tags realgh -run RealGH -v ./internal/forge/github/

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/forge"
)

func TestRealGHAttachesAnImageWithTheAgentToken(t *testing.T) {
	token, issue := os.Getenv("SWITCHBOARD_GH_TOKEN"), os.Getenv("SWITCHBOARD_GH_ISSUE")
	m := regexp.MustCompile(`^([^/]+)/([^#]+)#(\d+)$`).FindStringSubmatch(issue)
	if token == "" || m == nil {
		t.Skip("set SWITCHBOARD_GH_TOKEN and SWITCHBOARD_GH_ISSUE=owner/repo#n")
	}
	n, _ := strconv.Atoi(m[3])
	ref := forge.IssueRef{Forge: forge.GitHub, Owner: m[1], Repo: m[2], Number: n}
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for x := range 8 {
		img.Set(x, x, color.RGBA{R: 255, A: 255})
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "probe.png")
	if err := os.WriteFile(p, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	const last = "<!-- switchboard v=1 state=attachment -->"
	c, err := New(token).AttachComment(t.Context(), ref, "### V43 probe\n\n"+last+"\n", []string{p})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("comment %d:\n%s", c.ID, c.Body)
	if !strings.Contains(c.Body, "user-attachments") || !strings.HasSuffix(strings.TrimSpace(c.Body), last) {
		t.Fatalf("the comment has no attachment, or does not end with the marker:\n%s", c.Body)
	}
}
