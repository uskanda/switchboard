// Package shot takes a screenshot of a page served on this host with a local Chrome or Chromium, for the images of a
// plan question (ADR-069). It opens loopback pages only, and sends every other request of the page to a proxy that
// does not exist, so that a page, or the script a run gives it, cannot reach past this host.
package shot

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// ErrNoBrowser reports that no browser was configured and none of BrowserNames is on PATH.
var ErrNoBrowser = errors.New("no browser: set agent.browser, or put chrome-headless-shell, chromium, chromium-browser or google-chrome on PATH")

// BrowserNames are the names looked for on PATH, in order, when agent.browser is empty. The headless shell comes
// first: a full Chrome makes a Unix socket for its single instance, which a run's sandbox refuses, and does not start
// there (V58).
var BrowserNames = []string{"chrome-headless-shell", "chromium", "chromium-browser", "google-chrome"}

// loopback are the hosts a screenshot may open.
var loopback = []string{"localhost", "127.0.0.1", "::1"}

// CheckURL accepts an http or https URL whose host is localhost, 127.0.0.1 or ::1, and nothing else.
func CheckURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("not a URL: %q", raw)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%q is not an http or https URL", raw)
	}
	if u.User != nil {
		return fmt.Errorf("%q carries a user name", raw)
	}
	for _, h := range loopback {
		if u.Hostname() == h {
			return nil
		}
	}
	return fmt.Errorf("%q is not on localhost, 127.0.0.1 or ::1", raw)
}

// FindBrowser returns the browser to run: configured (agent.browser, ~/ expanded) when it is set, which must then be
// an executable file; else the first of BrowserNames that look finds; else ErrNoBrowser.
func FindBrowser(configured string, look func(string) (string, error)) (string, error) {
	if configured != "" {
		p := expandHome(configured)
		fi, err := os.Stat(p)
		if err != nil || fi.IsDir() || fi.Mode()&0o111 == 0 {
			return "", fmt.Errorf("agent.browser %s is not an executable file", configured)
		}
		return p, nil
	}
	if look == nil {
		look = exec.LookPath
	}
	for _, n := range BrowserNames {
		if p, err := look(n); err == nil {
			return p, nil
		}
	}
	return "", ErrNoBrowser
}

func expandHome(p string) string {
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			return home + "/" + rest
		}
	}
	return p
}

// Note is one mark drawn over the page before it is taken: a box around, or an arrow to, the element Selector finds or
// the Rect (x, y, width, height in CSS pixels of the page), with an optional Label.
type Note struct {
	Kind     string      `json:"kind"` // "box" or "arrow"
	Selector string      `json:"selector,omitempty"`
	Rect     *[4]float64 `json:"rect,omitempty"`
	Label    string      `json:"label,omitempty"`
}

// ParseNote reads a note given on the command line as "<selector or x,y,w,h>|<label>"; the label may be left out.
func ParseNote(kind, s string) (Note, error) {
	target, label, _ := strings.Cut(s, "|")
	target = strings.TrimSpace(target)
	if target == "" {
		return Note{}, fmt.Errorf("--%s %q names no element", kind, s)
	}
	n := Note{Kind: kind, Label: strings.TrimSpace(label)}
	if r, ok := parseRect(target); ok {
		n.Rect = &r
	} else {
		n.Selector = target
	}
	return n, nil
}

func parseRect(s string) ([4]float64, bool) {
	var r [4]float64
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		return r, false
	}
	for i, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return r, false
		}
		r[i] = f
	}
	return r, true
}

// Options are what to take.
type Options struct {
	URL string
	// Wait bounds how long the page is waited for to answer, as an app started in the same command takes a while to
	// listen; 30s when zero.
	Wait     time.Duration
	Width    int
	Height   int
	WaitFor  string        // a CSS selector to wait for before the script and the notes
	Settle   time.Duration // how long to wait after the page loaded (and WaitFor appeared), for what it fetches
	FullPage bool
	Script   string // JavaScript run in the page before it is taken
	Notes    []Note
	Timeout  time.Duration
}

// Take opens o.URL in browser, headless, and returns the PNG of the page.
func Take(ctx context.Context, browser string, o Options) ([]byte, error) {
	if err := CheckURL(o.URL); err != nil {
		return nil, err
	}
	for _, n := range o.Notes {
		if n.Kind != "box" && n.Kind != "arrow" {
			return nil, fmt.Errorf("a note is a box or an arrow, not %q", n.Kind)
		}
		if n.Selector == "" && n.Rect == nil {
			return nil, errors.New("a note names no element")
		}
	}
	if o.Width <= 0 {
		o.Width = 1280
	}
	if o.Height <= 0 {
		o.Height = 800
	}
	if o.Timeout <= 0 {
		o.Timeout = time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()
	if o.Wait <= 0 {
		o.Wait = 30 * time.Second
	}
	if err := waitServing(ctx, o.URL, o.Wait); err != nil {
		return nil, err
	}
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(browser),
		// The browser cannot build its own sandbox inside the run's sandbox (V58). It opens loopback pages only, and
		// everything else the page asks for, WebSockets and IP addresses included, goes to a proxy that is not there.
		chromedp.NoSandbox,
		chromedp.ProxyServer("http://127.0.0.1:9"),
		// <-loopback> drops the implicit bypass of all of 127.0.0.0/8 and *.localhost; only these three go direct.
		chromedp.Flag("proxy-bypass-list", "<-loopback>;localhost;127.0.0.1;[::1]"),
		chromedp.Flag("hide-scrollbars", true),
		chromedp.WindowSize(o.Width, o.Height),
	)
	actx, acancel := chromedp.NewExecAllocator(ctx, opts...)
	defer acancel()
	bctx, bcancel := chromedp.NewContext(actx)
	defer bcancel()

	steps := []chromedp.Action[chromedp.Void]{
		chromedp.EmulateViewport(int64(o.Width), int64(o.Height)),
		chromedp.Navigate(o.URL),
	}
	if o.WaitFor != "" {
		steps = append(steps, chromedp.WaitVisible(chromedp.CSS(o.WaitFor)))
	}
	if o.Settle > 0 {
		steps = append(steps, chromedp.Sleep(o.Settle))
	}
	if o.Script != "" {
		steps = append(steps, evaluate("the script", o.Script))
	}
	if len(o.Notes) > 0 {
		notes, err := json.Marshal(o.Notes)
		if err != nil {
			return nil, err
		}
		steps = append(steps, evaluate("the notes", "("+drawNotes+")("+string(notes)+")"))
	}
	if err := chromedp.Do(bctx, steps...); err != nil {
		return nil, err
	}
	if o.FullPage {
		return chromedp.Run(bctx, chromedp.FullScreenshot(100))
	}
	return chromedp.Run(bctx, chromedp.CaptureScreenshot())
}

// evaluate runs src in the page and waits for the promise it may return; an exception it throws is the error.
func evaluate(what, src string) chromedp.Action[chromedp.Void] {
	ev := chromedp.Evaluate[chromedp.Void](src, func(p *runtime.EvaluateParams) {
		p.AwaitPromise = new(true)
		p.UserGesture = new(true)
	})
	return func(ctx context.Context, t *chromedp.Target) (chromedp.Void, error) {
		v, err := ev(ctx, t)
		if err != nil {
			return v, fmt.Errorf("%s: %w", what, err)
		}
		return v, nil
	}
}

// drawNotes lays the notes over the page in one layer, in page coordinates, and throws when a selector finds nothing.
// The labels take the page's own font, so that Japanese text is drawn as the page draws it.
const drawNotes = `function (notes) {
  const color = "#e5484d";
  const layer = document.createElement("div");
  layer.setAttribute("data-switchboard-notes", "");
  Object.assign(layer.style, {position: "absolute", left: "0", top: "0", width: "0", height: "0",
    zIndex: "2147483647", pointerEvents: "none"});
  const ns = "http://www.w3.org/2000/svg";
  for (const n of notes) {
    let r;
    if (n.selector) {
      const el = document.querySelector(n.selector);
      if (!el) throw new Error("no element matches " + n.selector);
      const b = el.getBoundingClientRect();
      r = {x: b.left + scrollX, y: b.top + scrollY, w: b.width, h: b.height};
    } else {
      r = {x: n.rect[0], y: n.rect[1], w: n.rect[2], h: n.rect[3]};
    }
    let labelAt = {x: r.x, y: r.y - 6};
    if (n.kind === "box") {
      const box = document.createElement("div");
      Object.assign(box.style, {position: "absolute", left: (r.x - 4) + "px", top: (r.y - 4) + "px",
        width: (r.w + 8) + "px", height: (r.h + 8) + "px", border: "3px solid " + color, borderRadius: "6px",
        boxSizing: "border-box"});
      layer.appendChild(box);
    } else {
      // The arrow comes from above and to the left, or from below and to the right when there is no room above.
      const above = r.y >= 90;
      const from = above ? {x: Math.max(r.x - 90, 8), y: r.y - 70} : {x: r.x + r.w + 70, y: r.y + r.h + 60};
      const to = above ? {x: r.x + Math.min(r.w, 24) / 2, y: r.y}
        : {x: r.x + r.w - Math.min(r.w, 24) / 2, y: r.y + r.h};
      const svg = document.createElementNS(ns, "svg");
      const left = Math.min(from.x, to.x) - 12, top = Math.min(from.y, to.y) - 12;
      svg.setAttribute("width", Math.abs(to.x - from.x) + 24);
      svg.setAttribute("height", Math.abs(to.y - from.y) + 24);
      Object.assign(svg.style, {position: "absolute", left: left + "px", top: top + "px", overflow: "visible"});
      const line = document.createElementNS(ns, "line");
      line.setAttribute("x1", from.x - left); line.setAttribute("y1", from.y - top);
      line.setAttribute("x2", to.x - left); line.setAttribute("y2", to.y - top);
      line.setAttribute("stroke", color); line.setAttribute("stroke-width", "3");
      svg.appendChild(line);
      const a = Math.atan2(to.y - from.y, to.x - from.x), s = 12;
      const head = document.createElementNS(ns, "polygon");
      const p = (dx, dy) => (to.x - left + dx) + "," + (to.y - top + dy);
      head.setAttribute("points", [p(0, 0),
        p(-s * Math.cos(a - 0.45), -s * Math.sin(a - 0.45)),
        p(-s * Math.cos(a + 0.45), -s * Math.sin(a + 0.45))].join(" "));
      head.setAttribute("fill", color);
      svg.appendChild(head);
      layer.appendChild(svg);
      labelAt = above ? {x: from.x - 8, y: from.y - 4} : {x: from.x - 8, y: from.y + 28};
    }
    if (n.label) {
      const t = document.createElement("div");
      t.textContent = n.label;
      Object.assign(t.style, {position: "absolute", left: Math.max(labelAt.x, 4) + "px",
        top: Math.max(labelAt.y, 4) + "px", transform: "translateY(-100%)", background: color, color: "#fff",
        font: "600 14px/1.4 system-ui, sans-serif", padding: "2px 8px", borderRadius: "4px", whiteSpace: "nowrap"});
      layer.appendChild(t);
    }
  }
  document.body.appendChild(layer);
}`

// waitServing asks for rawURL, a loopback page CheckURL let through, until it answers with any status, for up to d.
// It goes straight to the page, without a proxy, and does not check a local certificate: it only waits for the app.
func waitServing(ctx context.Context, rawURL string, d time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, d)
	defer cancel()
	hc := &http.Client{
		Timeout:       2 * time.Second,
		Transport:     &http.Transport{Proxy: nil, TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, //nolint:gosec // a local app
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return err
		}
		if resp, err := hc.Do(req); err == nil {
			resp.Body.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%s does not answer after %v: start the app in the same command, before the picture", rawURL, d)
		case <-time.After(200 * time.Millisecond):
		}
	}
}
