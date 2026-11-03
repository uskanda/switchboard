package push

import (
	"errors"
	"io/fs"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/uskanda/switchboard/internal/state"
)

// MaxDevices is how many apps may be registered at once; registering one more drops the one registered longest ago.
const MaxDevices = 10

// MaxTokenBytes bounds a device token; FCM's are a few hundred bytes.
const MaxTokenBytes = 4096

// ErrInvalidToken reports an empty or too long device token.
var ErrInvalidToken = errors.New("the device token is empty or longer than 4096 bytes")

// devicesFile holds the registered device tokens in the state directory.
const devicesFile = "push-devices.json"

type device struct {
	Token        string    `json:"token"`
	RegisteredAt time.Time `json:"registered_at"`
}

// Devices are the apps the inbox service sends to, kept in the state directory across restarts.
type Devices struct {
	Dir state.Dir
	mu  sync.Mutex
}

func (d *Devices) read() ([]device, error) {
	var out []device
	if err := d.Dir.ReadJSON(devicesFile, &out); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return out, nil
}

// Register adds token, or renews its time when it is already there.
func (d *Devices) Register(token string, now time.Time) error {
	if strings.TrimSpace(token) == "" || len(token) > MaxTokenBytes {
		return ErrInvalidToken
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	all, err := d.read()
	if err != nil {
		return err
	}
	all = slices.DeleteFunc(all, func(x device) bool { return x.Token == token })
	all = append(all, device{Token: token, RegisteredAt: now.UTC()})
	slices.SortStableFunc(all, func(a, b device) int { return a.RegisteredAt.Compare(b.RegisteredAt) })
	if len(all) > MaxDevices {
		all = all[len(all)-MaxDevices:]
	}
	return d.Dir.WriteJSON(devicesFile, all)
}

// List returns the registered tokens, the one registered longest ago first.
func (d *Devices) List() ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	all, err := d.read()
	if err != nil {
		return nil, err
	}
	out := make([]string, len(all))
	for i, x := range all {
		out[i] = x.Token
	}
	return out, nil
}

// Remove drops token, which FCM no longer knows.
func (d *Devices) Remove(token string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	all, err := d.read()
	if err != nil {
		return err
	}
	return d.Dir.WriteJSON(devicesFile, slices.DeleteFunc(all, func(x device) bool { return x.Token == token }))
}
