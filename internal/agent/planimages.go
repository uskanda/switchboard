package agent

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/prompt"
	"github.com/uskanda/switchboard/internal/shot"
	"github.com/uskanda/switchboard/internal/workspace"
)

// planImagesOf says whether the plan runs of ref may show pictures (ADR-068), and how they take one: only a GitLab
// project's, and only when a browser is found. GitHub has no API that puts an image anywhere in a comment.
func (a *Agent) planImagesOf(ref forge.IssueRef, prototype bool) (prompt.PlanImages, bool) {
	if ref.Kind() != forge.GitLab {
		return prompt.PlanImages{}, false
	}
	if _, ok := a.forgeOf(ref).(forge.Uploader); !ok {
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
)

// attachPlanImages checks the images the questions of d name against the files collected from the run (dir), uploads
// the ones that pass with up, and keeps the Markdown each upload returned for questionText. An image that does not
// pass, or whose upload fails, is left out, logged as plan.image_dropped, and marked on its question. A question whose
// image is attached loses its diagram. With up nil (a GitHub project) the images are cleared without a word: such a run was
// never told about them.
func (a *Agent) attachPlanImages(ctx context.Context, ref forge.IssueRef, run string, d *PlanDeclaration, first int, dir string, up forge.Uploader) {
	posted := 0
	for i := range d.Questions {
		q := &d.Questions[i]
		n := first + i
		if up == nil {
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
			md, err := up.Upload(ctx, ref, p)
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
