package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/uskanda/switchboard/internal/claudecode"
	"github.com/uskanda/switchboard/internal/forge"
	ghforge "github.com/uskanda/switchboard/internal/forge/github"
	"github.com/uskanda/switchboard/internal/prompt"
	"github.com/uskanda/switchboard/internal/shot"
	"github.com/uskanda/switchboard/internal/workspace"
)

// planImagesOf says whether the plan runs of ref may show pictures (ADR-069, ADR-071), and how they take one: a GitLab
// project's when its client uploads, a GitHub project's when its client places attachments and gh has --attach, and
// only when a browser is found.
func (a *Agent) planImagesOf(ref forge.IssueRef, prototype bool) (prompt.PlanImages, bool) {
	switch f := a.forgeOf(ref); ref.Kind() {
	case forge.GitLab:
		if _, ok := f.(forge.Uploader); !ok {
			return prompt.PlanImages{}, false
		}
	case forge.GitHub:
		if _, ok := f.(forge.PlacedAttacher); !ok || !a.ghHasAttach() {
			return prompt.PlanImages{}, false
		}
	default:
		return prompt.PlanImages{}, false
	}
	find := a.FindBrowser
	if find == nil {
		find = func() (string, error) { return shot.FindBrowser(a.Config.Agent.Browser, nil) }
	}
	browser, err := find()
	if err != nil {
		return prompt.PlanImages{}, false
	}
	return prompt.PlanImages{Shot: shellWord(a.executable()) + " shot --browser " + shellWord(browser), Prototype: prototype}, true
}

// ghHasAttach reports whether gh has --attach: GHAttaches, or gh from PATH asked once.
func (a *Agent) ghHasAttach() bool {
	if a.GHAttaches != nil {
		return a.GHAttaches()
	}
	a.ghOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		v, err := ghforge.GHVersion(ctx, "")
		a.ghAttaches = err == nil && ghforge.GHAttaches(v)
	})
	return a.ghAttaches
}

// executable is the switchboard binary a plan run calls for its pictures.
func (a *Agent) executable() string {
	if a.Executable != "" {
		return a.Executable
	}
	exe, err := os.Executable()
	if err != nil {
		return "switchboard"
	}
	return exe
}

// shellWord quotes s for a shell when it holds anything but plain path characters.
func shellWord(s string) string {
	if s != "" && strings.Trim(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789/._-+~") == "" {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// Why an image a question named is not attached, for plan.image_dropped.
const (
	imageNoFile     = "no such file in " + workspace.PlanImageDir
	imageBadName    = "not a plain file name"
	imageBadType    = "not a PNG, JPEG or WebP image of at most 5 MB"
	imageTooMany    = "past the first 10 images of the round"
	imageBadLabel   = "not the label of an option"
	imageWithImage  = "the question has an image"
	imageUploadFail = "the upload failed: "
	imageAttachFail = "the attachment failed: "
)

// attachPlanImages checks the images the questions of d name against the files collected from the run (dir), hands
// the ones that pass to put, and keeps the Markdown put returned for questionText: an upload's, on GitLab, or a mark
// that a PlacedAttacher replaces, on GitHub. An image that does not pass, or that put fails, is left out, logged as
// plan.image_dropped, and marked on its question. A question whose image is attached loses its diagram. With put nil
// (a run never told about images) the images are cleared without a word.
func (a *Agent) attachPlanImages(ctx context.Context, ref forge.IssueRef, run string, d *PlanDeclaration, first int, dir string, put func(q int, option, path string) (string, error)) {
	posted := 0
	for i := range d.Questions {
		q := &d.Questions[i]
		n := first + i
		if put == nil {
			q.Image, q.OptionImages = "", nil
			continue
		}
		drop := func(option, name, why string) {
			q.imageDropped = true
			a.Audit.Log("plan.image_dropped", map[string]any{"issue": ref.String(), "run": run, "q": n, "option": option,
				"image": name, "reason": workspace.Redact(why)})
		}
		attach := func(option, name string) string {
			switch {
			case name != filepath.Base(name) || name == "." || name == ".." || strings.ContainsAny(name, `/\`):
				drop(option, name, imageBadName)
				return ""
			case posted == workspace.MaxScreenshots:
				drop(option, name, imageTooMany)
				return ""
			}
			p := filepath.Join(dir, name)
			fi, err := os.Lstat(p)
			if err != nil || !fi.Mode().IsRegular() {
				drop(option, name, imageNoFile)
				return ""
			}
			data, err := os.ReadFile(p)
			if err != nil {
				drop(option, name, imageNoFile)
				return ""
			}
			if ct, err := forge.ImageTypeUpTo(data, workspace.MaxScreenshotSize); err != nil || ct == "image/gif" {
				drop(option, name, imageBadType)
				return ""
			}
			md, err := put(n, option, p)
			if err != nil {
				drop(option, name, imageUploadFail+err.Error())
				return ""
			}
			posted++
			return md
		}
		if name := strings.TrimSpace(q.Image); name != "" {
			// The image takes the diagram's place; a diagram stays when the image cannot be attached.
			if q.imageMD = attach("", name); q.imageMD != "" && q.Diagram != "" {
				a.Audit.Log("plan.diagram_dropped", map[string]any{"issue": ref.String(), "run": run, "q": n, "reason": imageWithImage})
				q.Diagram = ""
			}
		}
		labels := make([]string, 0, len(q.OptionImages))
		for l := range q.OptionImages {
			labels = append(labels, l)
		}
		slices.Sort(labels)
		for _, l := range labels {
			name := strings.TrimSpace(q.OptionImages[l])
			if name == "" {
				continue
			}
			if !validLabel(l, len(q.Options)) {
				drop(l, name, imageBadLabel)
				continue
			}
			if md := attach(l, name); md != "" {
				if q.optionMD == nil {
					q.optionMD = map[string]string{}
				}
				q.optionMD[l] = md
			}
		}
	}
}

// validLabel says whether l labels one of n options.
func validLabel(l string, n int) bool {
	for i := range n {
		if optionLabel(i) == l {
			return true
		}
	}
	return false
}

// imageMark is the line a GitHub question comment holds where its image n goes, until AttachPlaced puts it there.
func imageMark(n int) string { return fmt.Sprintf("<!-- switchboard-image:%d -->", n) }

// placedImage is an image of a question comment that AttachPlaced puts in place: its mark, its file in the run
// directory, and the question and option it is the picture of, for plan.image_dropped.
type placedImage struct {
	Mark   string `json:"mark"`
	Path   string `json:"path"`
	Q      int    `json:"q"`
	Option string `json:"option,omitempty"`
}

// placePlanImages is attachPlanImages for a GitHub project: each image that passes gets a mark, kept in o.Placed, and
// o.Plain is the comment to post instead when the images cannot be put in place, where every question that had an
// image says so and keeps its diagram.
func (a *Agent) placePlanImages(ctx context.Context, ref forge.IssueRef, o *outcome, d *PlanDeclaration, first int, dir, word string, denials []claudecode.Denial, discarded bool) *outcome {
	plain := *d
	plain.Questions = slices.Clone(d.Questions)
	a.attachPlanImages(ctx, ref, o.Run, d, first, dir, func(q int, option, path string) (string, error) {
		m := placedImage{Mark: imageMark(len(o.Placed) + 1), Path: path, Q: q, Option: option}
		o.Placed = append(o.Placed, m)
		return m.Mark, nil
	})
	for i := range plain.Questions {
		q := d.Questions[i]
		plain.Questions[i].imageDropped = q.imageDropped || q.imageMD != "" || len(q.optionMD) > 0
	}
	if len(o.Placed) > 0 {
		po := *o
		po.Placed = nil
		o.Plain = po.planQuestions(&plain, first, denials, discarded, word).Body
	}
	return o.planQuestions(d, first, denials, discarded, word)
}
