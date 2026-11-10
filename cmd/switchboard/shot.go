package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/uskanda/switchboard/internal/cli"
	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/shot"
)

func init() {
	cli.Register("shot", runShot)
}

const shotUsage = `usage: switchboard shot --url URL --out FILE.png [options]

Takes a PNG of a page served on this host. Only http and https URLs on localhost, 127.0.0.1 and [::1] open, and the
page cannot load anything from another host. Start the app and take the picture in one command, since each sandboxed
command has a network of its own.

Options:
`

// runShot is switchboard shot: one picture of a loopback page, with an optional script run first and notes drawn over
// it (ADR-069).
func runShot(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("switchboard shot", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(stderr, shotUsage)
		fs.PrintDefaults()
	}
	var o shot.Options
	fs.StringVar(&o.URL, "url", "", "the page to take (localhost, 127.0.0.1 or [::1] only)")
	out := fs.String("out", "", "the PNG file to write")
	fs.IntVar(&o.Width, "width", 1280, "the width of the window in CSS pixels (390 for a phone)")
	fs.IntVar(&o.Height, "height", 800, "the height of the window in CSS pixels")
	fs.StringVar(&o.WaitFor, "wait", "", "a CSS selector to wait for before the script, the notes and the picture")
	fs.BoolVar(&o.FullPage, "full-page", false, "take the whole page, not only the window")
	fs.DurationVar(&o.Settle, "settle", 0, "how long to wait after the page loaded, for what it fetches (1500ms, for example)")
	script := fs.String("script", "", "a JavaScript file to run in the page before the picture, to show a change")
	eval := fs.String("eval", "", "JavaScript to run in the page before the picture (after --script)")
	notes := fs.String("notes", "", `a JSON file of notes: [{"kind":"box"|"arrow","selector":"#id"|"rect":[x,y,w,h],"label":"..."}]`)
	for _, kind := range []string{"box", "arrow"} {
		fs.Func(kind, "draw a "+kind+` at "SELECTOR|label" or "x,y,w,h|label"; may repeat`, func(v string) error {
			n, err := shot.ParseNote(kind, v)
			if err == nil {
				o.Notes = append(o.Notes, n)
			}
			return err
		})
	}
	browser := fs.String("browser", "", "the browser to run; agent.browser of --config, else chromium, chromium-browser or google-chrome on PATH")
	cfgPath := fs.String("config", config.DefaultPath(), "configuration file, read for agent.browser when --browser is not given")
	timeout := fs.Duration("timeout", time.Minute, "how long the picture may take")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if o.URL == "" || *out == "" || fs.NArg() > 0 {
		fs.Usage()
		return 2
	}
	if err := shot.CheckURL(o.URL); err != nil {
		return fail(stderr, err)
	}
	if *script != "" {
		data, err := os.ReadFile(*script)
		if err != nil {
			return fail(stderr, err)
		}
		o.Script = string(data)
	}
	if *eval != "" {
		if o.Script != "" {
			o.Script += ";\n"
		}
		o.Script += *eval
	}
	if *notes != "" {
		data, err := os.ReadFile(*notes)
		if err != nil {
			return fail(stderr, err)
		}
		var ns []shot.Note
		if err := json.Unmarshal(data, &ns); err != nil {
			return fail(stderr, fmt.Errorf("%s: %w", *notes, err))
		}
		o.Notes = append(ns, o.Notes...)
	}
	configured := *browser
	if configured == "" {
		if _, err := os.Stat(*cfgPath); err == nil {
			cfg, err := config.Load(*cfgPath)
			if err != nil {
				return fail(stderr, err)
			}
			configured = cfg.Agent.Browser
		}
	}
	exe, err := shot.FindBrowser(configured, nil)
	if err != nil {
		return fail(stderr, err)
	}
	o.Timeout = *timeout
	png, err := shot.Take(context.Background(), exe, o)
	if err != nil {
		return fail(stderr, err)
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		return fail(stderr, err)
	}
	if err := os.WriteFile(*out, png, 0o644); err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintf(stdout, "wrote %s\n", *out)
	return 0
}
