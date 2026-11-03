package push

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/state"
)

func TestRegisterKeepsDevicesAcrossRestarts(t *testing.T) {
	dir := state.Dir{Path: t.TempDir()}
	d := &Devices{Dir: dir}
	if err := d.Register("tok-a", at); err != nil {
		t.Fatal(err)
	}
	again := &Devices{Dir: dir}
	got, err := again.List()
	if err != nil || len(got) != 1 || got[0] != "tok-a" {
		t.Fatalf("List = %v, %v", got, err)
	}
}

func TestRegisteringAgainAddsNoDevice(t *testing.T) {
	d := &Devices{Dir: state.Dir{Path: t.TempDir()}}
	_ = d.Register("tok-a", at)
	_ = d.Register("tok-b", at.Add(time.Minute))
	_ = d.Register("tok-a", at.Add(2*time.Minute))
	got, _ := d.List()
	if len(got) != 2 {
		t.Fatalf("List = %v, want 2 devices", got)
	}
}

func TestTheEleventhDevicePushesOutTheOldest(t *testing.T) {
	d := &Devices{Dir: state.Dir{Path: t.TempDir()}}
	for i := 0; i < MaxDevices; i++ {
		_ = d.Register(fmt.Sprintf("tok-%d", i), at.Add(time.Duration(i)*time.Minute))
	}
	// tok-0 registers again, so tok-1 is now the oldest.
	_ = d.Register("tok-0", at.Add(time.Hour))
	_ = d.Register("tok-new", at.Add(2*time.Hour))
	got, _ := d.List()
	if len(got) != MaxDevices {
		t.Fatalf("%d devices, want %d", len(got), MaxDevices)
	}
	joined := strings.Join(got, ",")
	if strings.Contains(joined, "tok-1,") || !strings.Contains(joined, "tok-0") || !strings.Contains(joined, "tok-new") {
		t.Fatalf("devices = %v, want tok-1 removed", got)
	}
}

func TestRegisterRefusesEmptyAndLongTokens(t *testing.T) {
	d := &Devices{Dir: state.Dir{Path: t.TempDir()}}
	for _, tok := range []string{"", "   ", strings.Repeat("x", 4097)} {
		if err := d.Register(tok, at); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("Register(%d bytes) = %v, want ErrInvalidToken", len(tok), err)
		}
	}
	if got, _ := d.List(); len(got) != 0 {
		t.Fatalf("devices = %v", got)
	}
}

func TestRemoveDevice(t *testing.T) {
	d := &Devices{Dir: state.Dir{Path: t.TempDir()}}
	_ = d.Register("tok-a", at)
	_ = d.Register("tok-b", at)
	if err := d.Remove("tok-a"); err != nil {
		t.Fatal(err)
	}
	if got, _ := d.List(); len(got) != 1 || got[0] != "tok-b" {
		t.Fatalf("devices = %v", got)
	}
}
