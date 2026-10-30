package fake

import (
	"context"
	"sync"

	"github.com/uskanda/switchboard/internal/forge"
)

// Hooks is an in-memory forge.Hooks. Err, when set, is returned by every call, as a token without the right gets.
type Hooks struct {
	mu     sync.Mutex
	nextID int64
	Err    error
	hooks  map[string][]registered // by "owner/repo"
}

type registered struct {
	id   int64
	hook forge.Hook
}

// CreateHook implements forge.Hooks.
func (h *Hooks) CreateHook(_ context.Context, owner, repo string, hook forge.Hook) (int64, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.Err != nil {
		return 0, h.Err
	}
	if h.hooks == nil {
		h.hooks = map[string][]registered{}
	}
	h.nextID++
	h.hooks[owner+"/"+repo] = append(h.hooks[owner+"/"+repo], registered{h.nextID, hook})
	return h.nextID, nil
}

// DeleteHook implements forge.Hooks.
func (h *Hooks) DeleteHook(_ context.Context, owner, repo, url string) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.Err != nil {
		return false, h.Err
	}
	list := h.hooks[owner+"/"+repo]
	for i, r := range list {
		if r.hook.URL == url {
			h.hooks[owner+"/"+repo] = append(list[:i], list[i+1:]...)
			return true, nil
		}
	}
	return false, nil
}

// Of lists the webhooks registered on owner/repo.
func (h *Hooks) Of(owner, repo string) []forge.Hook {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []forge.Hook
	for _, r := range h.hooks[owner+"/"+repo] {
		out = append(out, r.hook)
	}
	return out
}
