//go:build realbrowser

// These tests drive a real browser: SWITCHBOARD_BROWSER, else one of BrowserNames on PATH.
//
//	SWITCHBOARD_BROWSER=~/.cache/ms-playwright/chromium-1243/chrome-linux64/chrome go test -tags realbrowser ./internal/shot/
package shot

import (
	"bytes"
	"context"
	"image/png"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func browser(t *testing.T) string {
	t.Helper()
	b, err := FindBrowser(os.Getenv("SWITCHBOARD_BROWSER"), nil)
	if err != nil {
		t.Skip(err)
	}
	return b
}

func page(t *testing.T, html string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(html))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func take(t *testing.T, o Options) ([]byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	return Take(ctx, browser(t), o)
}

func TestTakeWidth(t *testing.T) {
	u := page(t, `<!doctype html><body style="margin:0"><h1>こんにちは</h1></body>`)
	data, err := take(t, Options{URL: u, Width: 390, Height: 600})
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if w := img.Bounds().Dx(); w != 390 {
		t.Fatalf("width %d, want 390", w)
	}
}

// A request of the page to a host that is not one of the loopback names never arrives, and the page is still taken.
func TestTakeBlocksOtherHosts(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.2:0")
	if err != nil {
		t.Skip("cannot listen on 127.0.0.2:", err)
	}
	var hits atomic.Int32
	other := &httptest.Server{Listener: ln, Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
	})}}
	other.Start()
	t.Cleanup(other.Close)
	u := page(t, `<!doctype html><body><img src="`+other.URL+`/x.png"><script>
		fetch("`+other.URL+`/f").catch(() => {});
		try { new WebSocket("`+strings.Replace(other.URL, "http", "ws", 1)+`/ws"); } catch (e) {}
	</script><p id="done">ok</p></body>`)
	if _, err := take(t, Options{URL: u, WaitFor: "#done", Script: "new Promise(r => setTimeout(r, 500))"}); err != nil {
		t.Fatal(err)
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("the other host got %d request(s)", n)
	}
}

func TestTakeRefusesOtherURL(t *testing.T) {
	if _, err := Take(context.Background(), "/nonexistent/browser", Options{URL: "https://example.com/"}); err == nil ||
		!strings.Contains(err.Error(), "not on localhost") {
		t.Fatalf("err = %v, want the URL refused before the browser starts", err)
	}
}

func TestTakeScriptChangesThePage(t *testing.T) {
	u := page(t, `<!doctype html><body><button id="b">保存</button></body>`)
	before, err := take(t, Options{URL: u, Width: 400, Height: 200})
	if err != nil {
		t.Fatal(err)
	}
	after, err := take(t, Options{URL: u, Width: 400, Height: 200,
		Script: `document.getElementById("b").textContent = "保存して閉じる"`})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(before, after) {
		t.Fatal("the script did not change the image")
	}
}

func TestTakeScriptThrows(t *testing.T) {
	u := page(t, `<!doctype html><body>x</body>`)
	_, err := take(t, Options{URL: u, Script: `throw new Error("broken on purpose")`})
	if err == nil || !strings.Contains(err.Error(), "broken on purpose") {
		t.Fatalf("err = %v, want the exception", err)
	}
}

func TestTakeNotes(t *testing.T) {
	u := page(t, `<!doctype html><body style="padding:120px"><button id="save">保存</button></body>`)
	plain, err := take(t, Options{URL: u, Width: 500, Height: 300})
	if err != nil {
		t.Fatal(err)
	}
	marked, err := take(t, Options{URL: u, Width: 500, Height: 300, Notes: []Note{
		{Kind: "box", Selector: "#save", Label: "ここに移す"},
		{Kind: "arrow", Rect: &[4]float64{300, 200, 40, 20}, Label: "新しい列"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(plain, marked) {
		t.Fatal("the notes did not change the image")
	}
}

func TestTakeNoteWithoutElement(t *testing.T) {
	u := page(t, `<!doctype html><body>x</body>`)
	_, err := take(t, Options{URL: u, Notes: []Note{{Kind: "box", Selector: "#missing"}}})
	if err == nil || !strings.Contains(err.Error(), "#missing") {
		t.Fatalf("err = %v, want the missing element named", err)
	}
}
