package notify

import (
	"strings"
	"testing"
)

func TestWaitingMessage(t *testing.T) {
	got := Waiting{
		Ref: "gl:nas/switchboard-sandbox#6", Title: "Add a relay note", Reason: "question",
		Question: "Which word should I use: fast or quick?",
		IssueURL: "https://gl.test/nas/switchboard-sandbox/-/issues/6", WebURL: "http://192.168.0.82:8783",
	}.Text("ja")
	want := "⏸ 待ち（question）gl:nas/switchboard-sandbox#6\n" +
		"Add a relay note\n" +
		"質問：Which word should I use: fast or quick?\n" +
		"https://gl.test/nas/switchboard-sandbox/-/issues/6\n" +
		"受信箱：http://192.168.0.82:8783/issue/gl:nas%2Fswitchboard-sandbox%236"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestWaitingMessageWithoutWebOrQuestion(t *testing.T) {
	got := Waiting{Ref: "gh:o/r#1", Title: "t", Reason: "error", IssueURL: "https://github.test/o/r/issues/1"}.Text("en")
	want := "⏸ Waiting (error) gh:o/r#1\nt\nhttps://github.test/o/r/issues/1"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestWaitingMessageClipsTheQuestion(t *testing.T) {
	got := Waiting{Ref: "gh:o/r#1", Title: "t", Reason: "question", Question: strings.Repeat("あ", 400)}.Text("ja")
	line := strings.Split(got, "\n")[2]
	if n := len([]rune(strings.TrimPrefix(line, "質問："))); n != 300 || !strings.HasSuffix(line, "…") {
		t.Fatalf("question line has %d runes: %q", n, line)
	}
}

func TestDoneMessage(t *testing.T) {
	got := Done{Ref: "gh:o/r#5", ChangeURL: "https://github.test/o/r/pull/6", IssueURL: "https://github.test/o/r/issues/5"}.Text("ja")
	want := "✅ 完了 gh:o/r#5\nPR：https://github.test/o/r/pull/6\nhttps://github.test/o/r/issues/5"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	gl := Done{Ref: "gl:g/p#2", ChangeURL: "https://gl.test/g/p/-/merge_requests/3"}.Text("en")
	if gl != "✅ Done gl:g/p#2\nMR: https://gl.test/g/p/-/merge_requests/3" {
		t.Fatalf("got %q", gl)
	}
}
