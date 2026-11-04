package inbox

import (
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/marker"
)

func TestExcerptOfAPlanWaitIsThePlanTitle(t *testing.T) {
	long := strings.Repeat("設定", 40)
	for _, tc := range []struct{ reason, body, want string }{
		{marker.PlanQuestions, "### 計画についての質問\n\n**計画：** 設定の読み込みを分ける\n\nconfigを読みました。\n\n❓ **Q1** - **既定**\n\nどちら？\n\n➡️ A",
			"**計画：** 設定の読み込みを分ける"},
		{marker.PlanReady, "### 計画の承認のお願い\n\n**計画：** 設定の読み込みを分ける\n\n# 計画\n\n- 分ける。\n\n`/ok` と返答してください。",
			"**計画：** 設定の読み込みを分ける"},
		{marker.PlanReady, "### 計画の承認のお願い\n\n**計画：** " + long + "\n\n# 計画",
			string([]rune("**計画：** " + long)[:71]) + "…"},
		{marker.PlanReady, "### The plan is ready for approval\n\n# Plan\n\n- Add greeting.txt.\n\nReply `/ok`.", "- Add greeting.txt."},
	} {
		m := marker.Marker{V: 1, Host: "h", Run: "r", State: marker.Waiting, Reason: tc.reason}
		if got := Excerpt(tc.body + "\n\n" + m.Format() + "\n"); got != tc.want {
			t.Errorf("Excerpt(%s) = %q, want %q", tc.reason, got, tc.want)
		}
	}
}
