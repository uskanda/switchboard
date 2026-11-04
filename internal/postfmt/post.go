// Package postfmt holds the format of the agent's posts that more than one package reads: the agent writes them, the
// inbox quotes from them. Keeping it here lets the inbox read a post without depending on the agent.
package postfmt

import "strings"

// The labels that introduce the question in a waiting post, in each language the agent writes.
const (
	QuestionEnglish  = "Question"
	QuestionJapanese = "質問"
)

// QuestionLabels are the lines that introduce the question in the agent's posts, in every language.
func QuestionLabels() []string {
	return []string{"**" + QuestionEnglish + "**", "**" + QuestionJapanese + "**"}
}

// Question is the question in the text of an agent post (without its marker), on one line: the paragraph after the
// question label, or else the first paragraph after the heading. It is "" when there is none.
func Question(text string) string {
	for _, l := range QuestionLabels() {
		if i := strings.LastIndex(text, l+"\n"); i >= 0 {
			text = text[i+len(l):]
			break
		}
	}
	for _, para := range strings.Split(text, "\n\n") {
		p := strings.TrimSpace(para)
		if p == "" || strings.HasPrefix(p, "#") || (strings.HasPrefix(p, "**") && strings.HasSuffix(p, "**")) {
			continue
		}
		return strings.Join(strings.Fields(p), " ")
	}
	return ""
}
