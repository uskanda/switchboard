package agent

// texts are the fixed parts of what the agent posts, in one language (comments.language).
type texts struct {
	questionTitle, permissionTitle, errorTitle, verifyTitle, doneTitle string

	soFar, question, noQuestion, denied, reply string

	timeout, interrupted, maxTurns, failed, noDeclaration, prepare, start string
	configChanged, verifyFailed, notPushed, pushFailed, prFailed          string
	pr, verified, notVerified, prSummary, prVerification, withheld        string
}

var english = texts{
	questionTitle:   "The agent has a question",
	permissionTitle: "The agent was refused an operation",
	errorTitle:      "The run stopped",
	verifyTitle:     "The work did not pass the checks",
	doneTitle:       "The work is done",

	soFar:      "What was done",
	question:   "Question",
	noQuestion: "The run did not state a question. Reply with how to proceed.",
	denied:     "Refused operations",
	reply:      "Reply to this issue to run the agent again.",

	timeout:       "The run took longer than %s and was stopped.",
	interrupted:   "The run was interrupted: the agent stopped while it ran.",
	maxTurns:      "The run reached the turn limit (%d turns).",
	failed:        "claude ended with an error: %s.",
	noDeclaration: "The run ended without a valid declaration of its outcome: %s.",
	prepare:       "The worktree could not be prepared: %s",
	start:         "claude could not be started: %s",

	configChanged: "The Claude Code configuration in the worktree differs from the base branch, so claude was not started: %s. Restore these files, then reply.",
	verifyFailed:  "`%s` failed. The end of its output:",
	notPushed:     "The branch was not pushed:",
	pushFailed:    "The branch could not be pushed: %s",
	prFailed:      "The branch was pushed, but the pull request could not be opened: %s",

	pr:             "Pull request: %s",
	verified:       "`%s` passed.",
	notVerified:    "No verify_command is configured, so this change was not verified.",
	prSummary:      "Summary",
	prVerification: "Verification",
	withheld:       "The text was withheld because it looks like it contains a secret. It is kept in the agent's run directory for run %s.",
}

var japanese = texts{
	questionTitle:   "エージェントからの質問",
	permissionTitle: "エージェントの操作が拒否されました",
	errorTitle:      "実行が止まりました",
	verifyTitle:     "作業が検査を通りませんでした",
	doneTitle:       "作業が終わりました",

	soFar:      "ここまでの作業",
	question:   "質問",
	noQuestion: "実行は質問を残しませんでした。進め方を返信してください。",
	denied:     "拒否された操作",
	reply:      "このissueに返信すると、エージェントが再び実行します。",

	timeout:       "実行が制限時間（%s）を超えたため止めました。",
	interrupted:   "実行が中断されました。実行の途中でエージェントが止まりました。",
	maxTurns:      "実行がターン数の上限（%dターン）に達しました。",
	failed:        "claudeがエラーで終わりました：%s。",
	noDeclaration: "実行は、正しい終了の宣言をせずに終わりました：%s。",
	prepare:       "worktreeを用意できませんでした：%s",
	start:         "claudeを起動できませんでした：%s",

	configChanged: "worktreeのClaude Codeの設定が基準ブランチと異なるため、claudeを起動しませんでした：%s。これらのファイルを元に戻してから返信してください。",
	verifyFailed:  "`%s` が失敗しました。出力の末尾：",
	notPushed:     "ブランチをpushしませんでした：",
	pushFailed:    "ブランチをpushできませんでした：%s",
	prFailed:      "ブランチはpushしましたが、PRを作れませんでした：%s",

	pr:             "PR：%s",
	verified:       "`%s` が通りました。",
	notVerified:    "verify_commandが設定されていないため、この変更は検証していません。",
	prSummary:      "概要",
	prVerification: "検証",
	withheld:       "秘密情報を含むように見えるため、本文を伏せました。本文は、エージェントの実行ディレクトリ（実行 %s）に残っています。",
}

func textsFor(lang string) texts {
	if lang == "ja" {
		return japanese
	}
	return english
}
