package agent

import "strings"

// texts are the fixed parts of what the agent posts, in one language (comments.language).
type texts struct {
	questionTitle, permissionTitle, errorTitle, verifyTitle, doneTitle string

	soFar, question, noQuestion, denied, reply string

	timeout, interrupted, maxTurns, failed, noDeclaration, prepare, start  string
	configChanged, verifyFailed, notPushed, pushFailed, prFailed, mrFailed string
	pr, mr, verified, notVerified, prSummary, prVerification, withheld     string
	prUpdated, mrUpdated                                                   string // the done line of a reused change request
	claim, multipleStarts                                                  string

	// A push the agent does not fix: the title, why for each kind, and the heading of git's output.
	pushTitle, pushAuth, pushRule, pushMoved, gitOutput string
	// The claims of fix runs: of a CI failure (attempt, limit, checks), of a branch that moved on, and of a conflict
	// with the base (attempt, limit).
	fixCI, fixPush, fixConflict string
	// The wait after the CI failed for good: the title, the lead (change request, fixes made, limit; or the change
	// request alone when fixes are off), the headings, and the reply hint.
	ciTitle, ciFailed, ciNoFixes, ciChecks, ciAttempts, ciAttempt, ciLog, ciReply string
	// The wait after the change request kept conflicting with its base: the title and the lead, as for the CI.
	conflictTitle, conflictFailed, conflictNoFixes string

	// The queued comment: on the host's limit (host name, value) or on the project's (value).
	queuedHost, queuedProject string

	// The comments of the plan stage: the titles, the hints on how to answer, and the note on discarded changes.
	planQuestionsTitle, planTitle, planQuestionsHint, planHint, planDiscarded string

	// The progress comment of a run: its heading, the separators of a line, and a line withheld for a secret.
	progressTitle, toolSep, purposeSep, noTools, progressWithheld string
	hours, minutes                                                string // elapsed-time formats
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
	mrFailed:      "The branch was pushed, but the merge request could not be opened: %s",

	pr:             "Pull request: %s",
	prUpdated:      "Pull request updated: %s",
	mrUpdated:      "Merge request updated: %s",
	claim:          "Started on host %s.",
	queuedHost:     "Queued: host %s runs as many issues at once as it may (%d). This issue starts when a run ends.",
	queuedProject:  "Queued: this project runs as many issues at once as it may (%d). This issue starts when one of its runs ends.",
	multipleStarts: "More than one start label is on the issue (%s), so the agent cannot tell which run was asked for. Put exactly one of them on the issue to start again.",

	planQuestionsTitle: "The agent has questions for the plan",
	planTitle:          "The plan is ready for approval",
	planQuestionsHint:  "Reply `%s` to accept the recommended answers, or `%s q2 is B` to change some of them.",
	planHint:           "Reply `%s` to approve this plan and start the work. To change it, reply with what to change.",
	planDiscarded:      "The run changed the worktree although the plan stage changes nothing; those changes were discarded.",
	mr:                 "Merge request: %s",
	verified:           "`%s` passed.",
	notVerified:        "No verify_command is configured, so this change was not verified.",
	prSummary:          "Summary",
	prVerification:     "Verification",
	withheld:           "The text was withheld because it looks like it contains a secret. It is kept in the agent's run directory for run %s.",

	pushTitle: "The branch could not be pushed",
	pushAuth:  "The forge refused the agent's token: it failed to authenticate or may not write to the repository. Check that the agent's token is valid and may write the repository's contents, then reply.",
	pushRule:  "A rule of the remote refused the push: branch protection, a pre-receive hook, a push rule, or a size limit. Read the remote's message below. To have the agent change its commits to meet the rule, reply with what to change.",
	pushMoved: "The issue branch on the remote has commits the worktree does not, and the agent already made %d fix run(s) for this issue (the limit is %d). Merge or reset the branch, then reply.",
	gitOutput: "git's output",

	fixCI:       "The CI failed, so the agent fixes it (%d/%d): %s",
	fixPush:     "The push was refused because the issue branch on the remote moved on, so the agent merges it (%d/%d).",
	fixConflict: "The change request conflicts with the base branch, so the agent merges the base (%d/%d).",

	ciTitle:    "The CI still fails",
	ciFailed:   "The CI of %s failed again after %d automatic fix run(s); the limit is %d.",
	ciNoFixes:  "The CI of %s failed. Automatic fixes are off (agent.max_fix_attempts is 0).",
	ciChecks:   "Failed checks",
	ciAttempts: "Fix runs",
	ciAttempt:  "Fix %d",
	ciLog:      "End of the log of %s",
	ciReply:    "Reply to this issue to run the agent again; the count of automatic fixes then starts over.",

	conflictTitle:   "The change request still conflicts with the base branch",
	conflictFailed:  "%s still conflicts with the base branch after %d automatic fix run(s); the limit is %d.",
	conflictNoFixes: "%s conflicts with the base branch. Automatic fixes are off (agent.max_fix_attempts is 0).",

	progressTitle:    "Progress of the run",
	toolSep:          ", ",
	purposeSep:       " | ",
	noTools:          "no tools",
	progressWithheld: "(this line was withheld because it looks like it contains a secret)",
	hours:            "%dh%dm",
	minutes:          "%dm",
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
	mrFailed:      "ブランチはpushしましたが、MRを作れませんでした：%s",

	pr:             "PR：%s",
	prUpdated:      "PRを更新しました：%s",
	mrUpdated:      "MRを更新しました：%s",
	claim:          "端末%sで実行を始めます。",
	queuedHost:     "順番待ちにしました。端末%sで同時に動かせる実行の上限（%d）に達しています。実行が1つ終わると始めます。",
	queuedProject:  "順番待ちにしました。このプロジェクトで同時に動かせる実行の上限（%d）に達しています。このプロジェクトの実行が1つ終わると始めます。",
	multipleStarts: "始めるラベルが2つ以上付いているため（%s）、どの実行を求められたのか分かりません。どれか1つだけを付け直すと、もう一度始めます。",

	planQuestionsTitle: "計画についての質問",
	planTitle:          "計画の承認のお願い",
	planQuestionsHint:  "推奨どおりでよければ `%s`、変えたい問いがあれば `%s q2はB` のように返答してください。",
	planHint:           "この計画で進めるなら `%s` と返答してください。直してほしいところがあれば、そのまま返答してください。",
	planDiscarded:      "計画の段階では何も変えないはずの実行がworktreeを変えたため、その変更を捨てました。",
	mr:                 "MR：%s",
	verified:           "`%s` が通りました。",
	notVerified:        "verify_commandが設定されていないため、この変更は検証していません。",
	prSummary:          "概要",
	prVerification:     "検証",
	withheld:           "秘密情報を含むように見えるため、本文を伏せました。本文は、エージェントの実行ディレクトリ（実行 %s）に残っています。",

	pushTitle: "ブランチをpushできませんでした",
	pushAuth:  "フォージがエージェントのトークンを拒否しました。認証に失敗したか、リポジトリへの書き込みの権限がありません。エージェントのトークンが有効で、リポジトリの内容を書き込めることを確かめてから返信してください。",
	pushRule:  "リモートの規則がpushを拒否しました。ブランチの保護、pre-receive hook、push rule、サイズの上限のどれかです。下のリモートのメッセージを確かめてください。規則に合うようにエージェントにコミットを直させたいときは、直す内容を返信してください。",
	pushMoved: "リモートのissueのブランチに、worktreeに無いコミットがあります。このissueでは、エージェントは既に%d回直しました（上限は%d回）。ブランチを取り込むか戻してから返信してください。",
	gitOutput: "gitの出力",

	fixCI:       "CIが失敗したので直します（%d/%d）：%s",
	fixPush:     "リモートのissueのブランチが先に進んでいてpushが拒否されたので、取り込んで直します（%d/%d）。",
	fixConflict: "変更要求がbaseと衝突しているので、baseとの衝突を解消します（%d/%d）。",

	ciTitle:    "CIが失敗したままです",
	ciFailed:   "%sのCIが、%d回自動で直した後も失敗しました。上限は%d回です。",
	ciNoFixes:  "%sのCIが失敗しました。自動で直す設定は切ってあります（agent.max_fix_attemptsが0）。",
	ciChecks:   "失敗したチェック",
	ciAttempts: "直す実行",
	ciAttempt:  "%d回目",
	ciLog:      "%sのログの末尾",
	ciReply:    "このissueに返信すると、エージェントが再び実行します。自動で直す回数は数え直します。",

	conflictTitle:   "baseとの衝突が残っています",
	conflictFailed:  "%sは、%d回自動で直した後もbaseと衝突しています。上限は%d回です。",
	conflictNoFixes: "%sがbaseと衝突しています。自動で直す設定は切ってあります（agent.max_fix_attemptsが0）。",

	progressTitle:    "実行の進捗",
	toolSep:          "・",
	purposeSep:       " ｜ ",
	noTools:          "ツールなし",
	progressWithheld: "（秘密情報を含むように見えるため、この行を伏せました）",
	hours:            "%d時間%d分",
	minutes:          "%d分",
}

func textsFor(lang string) texts {
	if lang == "ja" {
		return japanese
	}
	return english
}

// QuestionLabels are the lines that introduce the question in the agent's posts, in every language. The inbox
// uses them to quote the question.
func QuestionLabels() []string {
	return []string{"**" + english.question + "**", "**" + japanese.question + "**"}
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
