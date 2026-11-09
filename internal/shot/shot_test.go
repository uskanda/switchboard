package shot

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCheckURL(t *testing.T) {
	ok := []string{
		"http://localhost:5173/", "http://127.0.0.1/x?y=1", "https://localhost/", "http://[::1]:8080/inbox",
	}
	for _, u := range ok {
		if err := CheckURL(u); err != nil {
			t.Errorf("CheckURL(%q) = %v, want nil", u, err)
		}
	}
	bad := []string{
		"https://example.com/", "http://127.0.0.2/", "http://localhost.example.com/", "file:///etc/passwd",
		"javascript:alert(1)", "ftp://localhost/", "http://user@example.com@localhost/", "", "localhost:5173",
		"http://10.0.0.1/", "http://0.0.0.0/",
	}
	for _, u := range bad {
		if err := CheckURL(u); err == nil {
			t.Errorf("CheckURL(%q) = nil, want an error", u)
		}
	}
}

func TestFindBrowserConfigured(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "chrome")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := FindBrowser(exe, func(string) (string, error) { return "", errors.New("not on PATH") })
	if err != nil || got != exe {
		t.Fatalf("FindBrowser(%q) = %q, %v", exe, got, err)
	}
	if _, err := FindBrowser(filepath.Join(dir, "missing"), nil); err == nil {
		t.Fatal("a configured path that does not exist must be an error")
	}
}

func TestFindBrowserPathOrder(t *testing.T) {
	var asked []string
	look := func(name string) (string, error) {
		asked = append(asked, name)
		if name == "chromium" {
			return "/usr/bin/google-chrome", nil
		}
		return "", errors.New("not found")
	}
	got, err := FindBrowser("", look)
	if err != nil || got != "/usr/bin/google-chrome" {
		t.Fatalf("FindBrowser = %q, %v", got, err)
	}
	// The headless shell comes first: a full Chrome cannot start inside a run's sandbox (V58).
	if want := []string{"chrome-headless-shell", "chromium"}; len(asked) != 2 || asked[0] != want[0] || asked[1] != want[1] {
		t.Fatalf("asked %v, want %v", asked, want)
	}
}

func TestFindBrowserNone(t *testing.T) {
	_, err := FindBrowser("", func(string) (string, error) { return "", errors.New("not found") })
	if !errors.Is(err, ErrNoBrowser) {
		t.Fatalf("err = %v, want ErrNoBrowser", err)
	}
}

func TestParseNote(t *testing.T) {
	n, err := ParseNote("box", "#save|ここに移す")
	if err != nil || n.Kind != "box" || n.Selector != "#save" || n.Label != "ここに移す" {
		t.Fatalf("ParseNote = %+v, %v", n, err)
	}
	n, err = ParseNote("arrow", "10,20,30,40|新しい列")
	if err != nil || n.Rect == nil || *n.Rect != [4]float64{10, 20, 30, 40} || n.Label != "新しい列" {
		t.Fatalf("ParseNote rect = %+v, %v", n, err)
	}
	if _, err := ParseNote("box", ""); err == nil {
		t.Fatal("an empty note must be an error")
	}
}

// The page is waited for until it answers: an app started in the same command as the picture may take a while.
func TestWaitServingWaitsForTheApp(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	go func() {
		time.Sleep(400 * time.Millisecond)
		l, err := net.Listen("tcp", addr)
		if err != nil {
			return
		}
		_ = http.Serve(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) }))
	}()
	start := time.Now()
	if err := waitServing(context.Background(), "http://"+addr+"/page", 5*time.Second); err != nil {
		t.Fatalf("waitServing = %v", err)
	}
	if time.Since(start) < 300*time.Millisecond {
		t.Fatal("waitServing did not wait for the app")
	}
}

func TestWaitServingGivesUp(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	err = waitServing(context.Background(), "http://"+addr+"/", 300*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "does not answer") {
		t.Fatalf("err = %v", err)
	}
}
