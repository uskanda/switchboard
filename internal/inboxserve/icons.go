package inboxserve

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
)

// IconTTL is how long an image read from a forge or an icon URL is served without reading it again.
const IconTTL = 24 * time.Hour

// IconFailTTL is how long a failed read is remembered, so that a forge is not asked again on every page.
const IconFailTTL = time.Hour

// IconFetchTimeout bounds one read of an image.
const IconFetchTimeout = 15 * time.Second

// errNoIcon reports a project whose icon is an emoji, which the page shows as text.
var errNoIcon = errors.New("the icon is an emoji, not an image")

// Icons serves /v1/project-icon (a project's icon as an image) and /v1/project-icons (the emoji icons). An icon is the
// project's icon in the configuration, else the forge's avatar read with the inbox token (ADR-048).
type Icons struct {
	// Config is the configuration in use; Server.Reload replaces it.
	Config *config.Config
	// Avatars reads avatars with each forge's inbox token, by forge kind.
	Avatars map[string]forge.Avatars
	// Dir keeps the images read, one file each, so that a restart does not read them again; nothing is kept on disk
	// when it is "".
	Dir string
	// HTTP reads icon URLs, without a token; a client with IconFetchTimeout when nil.
	HTTP *http.Client

	mu   sync.Mutex
	read map[string]iconRead // by cacheKey
}

type iconRead struct {
	data []byte
	at   time.Time
	err  error
}

func (s *Server) iconConfig() *config.Config {
	s.vmu.RLock()
	defer s.vmu.RUnlock()
	return s.Icons.Config
}

// projectIcon serves GET /v1/project-icon?ref=: the image, or 404 when the project has none that can be read.
func (s *Server) projectIcon(w http.ResponseWriter, r *http.Request) {
	ref := r.URL.Query().Get("ref")
	p, ok := findProject(s.iconConfig(), ref)
	if !ok {
		fail(w, http.StatusNotFound, inboxapi.CodeNotFound, ref+" is not a project in the configuration")
		return
	}
	data, err := s.Icons.image(r.Context(), p, s.now())
	if err != nil {
		fail(w, http.StatusNotFound, inboxapi.CodeNotFound, "no icon for "+p.Ref+": "+err.Error())
		return
	}
	ct, _ := forge.ImageType(data)
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "max-age=3600")
	w.Write(data)
}

// projectIcons serves GET /v1/project-icons: the emoji icons of the configuration, by project reference.
func (s *Server) projectIcons(w http.ResponseWriter, _ *http.Request) {
	out := inboxapi.ProjectIcons{}
	for _, p := range s.iconConfig().Projects {
		if p.IconKind() == config.IconEmoji {
			out[p.Ref] = p.Icon
		}
	}
	send(w, http.StatusOK, out)
}

func findProject(cfg *config.Config, ref string) (config.Project, bool) {
	for _, p := range cfg.Projects {
		if p.Ref == ref {
			return p, true
		}
	}
	for _, p := range cfg.Projects {
		if strings.EqualFold(p.Ref, ref) {
			return p, true
		}
	}
	return config.Project{}, false
}

// image is p's icon as an image checked by forge.ImageType. A file is read each time; an image from a URL or a forge
// is kept for IconTTL, and a failure to read one for IconFailTTL.
func (ic *Icons) image(ctx context.Context, p config.Project, now time.Time) ([]byte, error) {
	switch p.IconKind() {
	case config.IconEmoji:
		return nil, errNoIcon
	case config.IconFile:
		return readImage(p.Icon)
	}
	key := cacheKey(p)
	ic.mu.Lock()
	r, ok := ic.read[key]
	ic.mu.Unlock()
	if ok && (r.err == nil && now.Sub(r.at) < IconTTL || r.err != nil && now.Sub(r.at) < IconFailTTL) {
		return r.data, r.err
	}
	if data, at, ok := ic.fromDisk(key, now); ok {
		ic.keep(key, iconRead{data: data, at: at})
		return data, nil
	}
	// The read is cut loose from the request: a page that goes away must not leave a failure remembered for an hour.
	fctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), IconFetchTimeout)
	defer cancel()
	data, err := ic.fetch(fctx, p)
	if err == nil {
		_, err = forge.ImageType(data)
	}
	if err != nil {
		ic.keep(key, iconRead{at: now, err: err})
		return nil, err
	}
	ic.keep(key, iconRead{data: data, at: now})
	ic.toDisk(key, data, now)
	return data, nil
}

func (ic *Icons) fetch(ctx context.Context, p config.Project) ([]byte, error) {
	if p.IconKind() == config.IconURL {
		hc := ic.HTTP
		if hc == nil {
			hc = &http.Client{Timeout: IconFetchTimeout}
		}
		return forge.GetImage(ctx, hc, p.Icon, "", nil)
	}
	a := ic.Avatars[p.Forge]
	if a == nil {
		return nil, forge.ErrNotFound
	}
	return a.ProjectAvatar(ctx, p.Owner, p.Repo)
}

func (ic *Icons) keep(key string, r iconRead) {
	ic.mu.Lock()
	defer ic.mu.Unlock()
	if ic.read == nil {
		ic.read = map[string]iconRead{}
	}
	ic.read[key] = r
}

// cacheKey names what an image was read from: the project and its icon, so that a changed icon is read anew.
func cacheKey(p config.Project) string {
	sum := sha256.Sum256([]byte(p.Ref + "\n" + p.Icon))
	return hex.EncodeToString(sum[:])
}

// fromDisk is the image kept in Dir under key while it is younger than IconTTL, and when it was read.
func (ic *Icons) fromDisk(key string, now time.Time) ([]byte, time.Time, bool) {
	if ic.Dir == "" {
		return nil, time.Time{}, false
	}
	path := filepath.Join(ic.Dir, key)
	st, err := os.Stat(path)
	if err != nil || now.Sub(st.ModTime()) >= IconTTL {
		return nil, time.Time{}, false
	}
	data, err := readImage(path)
	if err != nil {
		return nil, time.Time{}, false
	}
	return data, st.ModTime(), true
}

// toDisk keeps data in Dir under key, its time set to when it was read. A failure only loses the copy on disk.
func (ic *Icons) toDisk(key string, data []byte, now time.Time) {
	if ic.Dir == "" || os.MkdirAll(ic.Dir, 0o700) != nil {
		return
	}
	f, err := os.CreateTemp(ic.Dir, ".icon-*")
	if err != nil {
		return
	}
	_, werr := f.Write(data)
	cerr := f.Close()
	path := filepath.Join(ic.Dir, key)
	if werr != nil || cerr != nil || os.Chtimes(f.Name(), now, now) != nil || os.Rename(f.Name(), path) != nil {
		os.Remove(f.Name())
	}
}

// readImage reads an image file of at most forge.MaxImageBytes and checks it.
func readImage(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, forge.MaxImageBytes+1))
	if err != nil {
		return nil, err
	}
	if _, err := forge.ImageType(data); err != nil {
		return nil, err
	}
	return data, nil
}
