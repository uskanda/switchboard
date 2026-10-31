package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/uskanda/switchboard/internal/cli"
	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/editor"
	"github.com/uskanda/switchboard/internal/instructions"
)

func init() {
	cli.Register("instructions edit", instructionsEdit)
}

// instructionsEdit opens the additional instructions of the plan runs in $EDITOR: the global ones, or a project's.
func instructionsEdit(args []string, stdout, stderr io.Writer) int {
	cfg, fs, code := parse("instructions edit", args, stderr, nil)
	if code >= 0 {
		return code
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(stderr, "usage: switchboard instructions edit [gh:owner/repo|gl:group/project] [--config FILE]")
		return 2
	}
	target := fs.Arg(0)
	if target != instructions.Global && !configured(cfg, target) {
		return fail(stderr, fmt.Errorf("%s is not a project in the configuration", target))
	}
	d, err := stateDir()
	if err != nil {
		return fail(stderr, err)
	}
	s := instructions.For(cfg.Path, d.Audit())
	path, err := s.Path(target)
	if err != nil {
		return fail(stderr, err)
	}
	old, err := s.Read(target)
	if err != nil {
		return fail(stderr, err)
	}
	draft, err := os.CreateTemp("", "switchboard-instructions-*.md")
	if err != nil {
		return fail(stderr, err)
	}
	defer os.Remove(draft.Name())
	if _, err := draft.WriteString(old); err != nil {
		draft.Close()
		return fail(stderr, err)
	}
	if err := draft.Close(); err != nil {
		return fail(stderr, err)
	}
	if err := editor.Run(editor.Default(), draft.Name()); err != nil {
		return fail(stderr, err)
	}
	text, err := os.ReadFile(draft.Name())
	if err != nil {
		return fail(stderr, err)
	}
	if string(text) == old {
		fmt.Fprintf(stdout, "%s is unchanged\n", path)
		return 0
	}
	if err := s.Save(target, string(text)); err != nil {
		if errors.Is(err, instructions.ErrTooLarge) || errors.Is(err, instructions.ErrSecret) {
			err = fmt.Errorf("%w; %s was not changed", err, path)
		}
		return fail(stderr, err)
	}
	fmt.Fprintf(stdout, "saved %s\n", path)
	return 0
}

// configured reports whether ref is one of the configured projects, on any host.
func configured(cfg *config.Config, ref string) bool {
	return slices.ContainsFunc(cfg.Projects, func(p config.Project) bool { return p.Ref == ref })
}
