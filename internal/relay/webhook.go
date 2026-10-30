package relay

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/uskanda/switchboard/internal/config"
)

const (
	maxBody     = 5 << 20        // 5 MiB
	rememberFor = 72 * time.Hour // how long a delivery ID is remembered
	maxRemember = 100_000        // the most delivery IDs remembered at once
)

// Receiver takes the webhooks at /webhook/{github|gitlab}/<hook-key> (design.md 3). It reads the secrets with Secret,
// passes each new verified event to Publish, and logs only IDs and outcomes with Logf.
type Receiver struct {
	Hooks   []config.Hook
	Secret  func(ref string) (string, error)
	Now     func() time.Time
	Publish func(Event)
	Logf    func(format string, args ...any)

	mu   sync.Mutex
	seen map[string]time.Time // delivery ID -> when it was first taken
}

func (r *Receiver) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	rest, ok := strings.CutPrefix(req.URL.Path, "/webhook/")
	forge, key, ok2 := strings.Cut(rest, "/")
	if !ok || !ok2 || strings.Contains(key, "/") {
		http.NotFound(w, req)
		return
	}
	if req.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	hook, found := r.hook(key)
	if !found || (forge == "github") != (hook.Forge == "gh") || (forge != "github" && forge != "gitlab") {
		r.Logf("webhook: refused: unknown hook key on /webhook/%s", forge)
		http.NotFound(w, req)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, req.Body, maxBody))
	if err != nil {
		r.Logf("webhook %s: refused: the body is over %d bytes", hook.Project, maxBody)
		http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
		return
	}
	secret, err := r.Secret(hook.SecretRef)
	if err != nil {
		r.Logf("webhook %s: cannot read the secret %s: %v", hook.Project, hook.SecretRef, err)
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	var delivery string
	var ev Event
	var follow bool
	h := req.Header
	if hook.Forge == "gh" {
		delivery = h.Get("X-GitHub-Delivery")
		err = verifyGitHub([]byte(secret), h.Get("X-Hub-Signature-256"), body)
	} else if hook.GitLabToken {
		delivery = h.Get("Idempotency-Key")
		err = verifyGitLabToken(secret, h.Get("X-Gitlab-Token"))
	} else {
		delivery = h.Get("webhook-id")
		err = verifyGitLab(secret, delivery, h.Get("webhook-timestamp"), h.Get("webhook-signature"), body, r.Now())
	}
	if err != nil {
		r.Logf("webhook %s delivery %q: refused: %v", hook.Project, delivery, err)
		http.Error(w, "signature does not verify", http.StatusUnauthorized)
		return
	}
	if delivery == "" || !r.first(delivery) {
		if delivery != "" {
			r.Logf("webhook %s delivery %s: duplicate", hook.Project, delivery)
		}
		w.WriteHeader(http.StatusOK)
		return
	}
	if hook.Forge == "gh" {
		ev, follow, err = extractGitHub(h.Get("X-GitHub-Event"), body)
	} else {
		ev, follow, err = extractGitLab(h.Get("X-Gitlab-Event"), body)
	}
	switch {
	case err != nil:
		r.Logf("webhook %s delivery %s: refused: %v", hook.Project, delivery, err)
		http.Error(w, "bad payload", http.StatusBadRequest)
		return
	case !follow:
		r.Logf("webhook %s delivery %s: not followed", hook.Project, delivery)
		w.WriteHeader(http.StatusOK)
		return
	case !strings.EqualFold(ev.Project, hook.FullName()):
		r.Logf("webhook %s delivery %s: refused: the payload is of another project", hook.Project, delivery)
		http.Error(w, "another project", http.StatusBadRequest)
		return
	}
	ev.Delivery = delivery
	r.Logf("webhook %s#%d delivery %s: %s, sent", ev.Project, ev.Issue, delivery, ev.Kind)
	r.Publish(ev)
	w.WriteHeader(http.StatusOK)
}

func (r *Receiver) hook(key string) (config.Hook, bool) {
	for _, h := range r.Hooks {
		if h.Key == key {
			return h, true
		}
	}
	return config.Hook{}, false
}

// first records delivery and reports whether it is new within rememberFor.
func (r *Receiver) first(delivery string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.Now()
	if r.seen == nil {
		r.seen = map[string]time.Time{}
	}
	if at, ok := r.seen[delivery]; ok && now.Sub(at) < rememberFor {
		return false
	}
	if len(r.seen) >= maxRemember {
		r.forget(now)
	}
	r.seen[delivery] = now
	return true
}

// forget drops the IDs older than rememberFor, and then the oldest until there is room.
func (r *Receiver) forget(now time.Time) {
	for id, at := range r.seen {
		if now.Sub(at) >= rememberFor {
			delete(r.seen, id)
		}
	}
	for len(r.seen) >= maxRemember {
		var oldest string
		var at time.Time
		for id, t := range r.seen {
			if oldest == "" || t.Before(at) {
				oldest, at = id, t
			}
		}
		delete(r.seen, oldest)
	}
}
