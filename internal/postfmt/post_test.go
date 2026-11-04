package postfmt

import "testing"

func TestQuestion(t *testing.T) {
	cases := map[string]string{
		"### The agent has a question\n\nRead the parser.\n\n**Question**\nOn or\noff?\n\nReply here.": "On or off?",
		"### エージェントからの質問\n\n**質問**\n既定はオフですか？":                                                         "既定はオフですか？",
		"### The agent stopped\n\nThe run timed out.":                                                  "The run timed out.",
		"### Only a heading": "",
	}
	for text, want := range cases {
		if got := Question(text); got != want {
			t.Errorf("Question(%q) = %q, want %q", text, got, want)
		}
	}
}
