package notify

import (
	"net/url"
	"strings"
)

// Waiting is what a waiting notification says about an issue.
type Waiting struct {
	Ref, Title, Reason string
	Question           string // the question in the agent's post, on one line; may be empty
	IssueURL           string
	WebURL             string // the web inbox; may be empty
}

// Done is what a done notification says about an issue.
type Done struct {
	Ref       string
	ChangeURL string // the pull or merge request the run opened; may be empty
	IssueURL  string
}

// questionRunes caps the question excerpt so a notification stays short.
const questionRunes = 300

// Text renders the notification in lang ("ja" or "en"), one item per line, skipping the empty ones.
func (w Waiting) Text(lang string) string {
	head, question, inbox := "⏸ Waiting ("+w.Reason+") ", "Question: ", "Inbox: "
	if lang == "ja" {
		head, question, inbox = "⏸ 待ち（"+w.Reason+"）", "質問：", "受信箱："
	}
	lines := []string{head + w.Ref, w.Title}
	if w.Question != "" {
		lines = append(lines, question+clip(w.Question, questionRunes))
	}
	lines = append(lines, w.IssueURL)
	if w.WebURL != "" {
		lines = append(lines, inbox+strings.TrimRight(w.WebURL, "/")+"/issue/"+url.PathEscape(w.Ref))
	}
	return join(lines)
}

// Text renders the notification in lang ("ja" or "en").
func (d Done) Text(lang string) string {
	head, sep := "✅ Done ", ": "
	if lang == "ja" {
		head, sep = "✅ 完了 ", "："
	}
	lines := []string{head + d.Ref}
	if d.ChangeURL != "" {
		kind := "PR"
		if strings.HasPrefix(d.Ref, "gl:") {
			kind = "MR"
		}
		lines = append(lines, kind+sep+d.ChangeURL)
	}
	return join(append(lines, d.IssueURL))
}

func join(lines []string) string {
	var out []string
	for _, l := range lines {
		if l != "" {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
