package inboxserve

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
)

// IssueImages serves /v1/issue-image: an image uploaded to a GitLab project, as the question comments of plan runs
// hold them (ADR-069), read with the inbox token. An upload never changes, so the image read is kept in Dir for good.
type IssueImages struct {
	// Config is the configuration in use; Server.Reload replaces it.
	Config *config.Config
	// Uploads reads the uploads with each forge's inbox token, by forge kind. Only GitLab has one.
	Uploads map[string]forge.UploadReader
	// Dir keeps the images read; nothing is kept on disk when it is "".
	Dir string
}

// uploadSecret is the form of the secret part of an upload's path.
var uploadSecret = regexp.MustCompile(`^[0-9a-f]{16,64}$`)

func (s *Server) issueImagesConfig() *config.Config {
	s.vmu.RLock()
	defer s.vmu.RUnlock()
	return s.IssueImages.Config
}

// issueImage serves GET /v1/issue-image?issue=&src=: the image src points to, when it is an upload of the issue's
// GitLab project; 404 for anything else, before any request leaves.
func (s *Server) issueImage(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	issue, src := q.Get("issue"), q.Get("src")
	ref, _, _ := strings.Cut(issue, "#")
	p, ok := findProject(s.issueImagesConfig(), ref)
	if ok && (!s.sees(r.Context(), p.Ref) || p.Forge != forge.GitLab) {
		ok = false
	}
	up := s.IssueImages.Uploads[p.Forge]
	if !ok || up == nil {
		fail(w, http.StatusNotFound, inboxapi.CodeNotFound, "no images for "+ref)
		return
	}
	secret, name, ok := uploadOf(src, s.issueImagesConfig().Forges.GitLab.BaseURL, p.Owner, p.Repo)
	if !ok {
		fail(w, http.StatusNotFound, inboxapi.CodeNotFound, "not an upload of "+p.Ref)
		return
	}
	data, err := s.IssueImages.read(r.Context(), up, p, secret, name)
	if err != nil {
		fail(w, http.StatusNotFound, inboxapi.CodeNotFound, "the image cannot be read: "+err.Error())
		return
	}
	ct, _ := forge.ImageTypeUpTo(data, forge.MaxQuestionImageBytes)
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Write(data)
}

// uploadOf reads src as an upload of owner/repo: "/uploads/<secret>/<name>", as GitLab writes it in Markdown, or the
// same under the project's address on the instance at base. The name is one path segment.
func uploadOf(src, base, owner, repo string) (secret, name string, ok bool) {
	u, err := url.Parse(src)
	if err != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", "", false
	}
	path := u.EscapedPath()
	switch {
	case u.Scheme == "" && u.Host == "" && strings.HasPrefix(path, "/uploads/"):
		path = strings.TrimPrefix(path, "/uploads/")
	case u.Scheme == "https" || u.Scheme == "http":
		b, err := url.Parse(base)
		if err != nil || b.Host == "" || !strings.EqualFold(u.Host, b.Host) || u.User != nil {
			return "", "", false
		}
		prefix := strings.TrimRight(b.EscapedPath(), "/") + "/" + owner + "/" + repo + "/uploads/"
		if !strings.HasPrefix(path, prefix) {
			return "", "", false
		}
		path = strings.TrimPrefix(path, prefix)
	default:
		return "", "", false
	}
	secret, rawName, found := strings.Cut(path, "/")
	if !found || !uploadSecret.MatchString(secret) || strings.Contains(rawName, "/") {
		return "", "", false
	}
	name, err = url.PathUnescape(rawName)
	if err != nil || name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00") {
		return "", "", false
	}
	return secret, name, true
}

// read is the upload, from Dir when it was read before.
func (ii *IssueImages) read(ctx context.Context, up forge.UploadReader, p config.Project, secret, name string) ([]byte, error) {
	sum := sha256.Sum256([]byte(p.Ref + "\n" + secret + "\n" + name))
	path := ""
	if ii.Dir != "" {
		path = filepath.Join(ii.Dir, hex.EncodeToString(sum[:]))
		if data, err := os.ReadFile(path); err == nil {
			if _, err := forge.ImageTypeUpTo(data, forge.MaxQuestionImageBytes); err == nil {
				return data, nil
			}
		}
	}
	fctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), IconFetchTimeout)
	defer cancel()
	data, err := up.ReadUpload(fctx, p.Owner, p.Repo, secret, name)
	if err != nil {
		return nil, err
	}
	if _, err := forge.ImageTypeUpTo(data, forge.MaxQuestionImageBytes); err != nil {
		return nil, errors.New("not a PNG, JPEG, GIF or WebP image of at most 5 MB")
	}
	if path != "" && os.MkdirAll(ii.Dir, 0o700) == nil {
		if f, err := os.CreateTemp(ii.Dir, ".image-*"); err == nil {
			_, werr := f.Write(data)
			if cerr := f.Close(); werr != nil || cerr != nil || os.Rename(f.Name(), path) != nil {
				os.Remove(f.Name())
			}
		}
	}
	return data, nil
}
