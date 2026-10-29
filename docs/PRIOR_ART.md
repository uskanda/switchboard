# 類似事例・調査メモ（2026-10 時点）

採用ではなく設計の参考として記録する。

## 公式機能でできること・できないこと

- Pro/Max の個人サブスクでは、全セッションを横断取得する公式 API はない。Claude Code Analytics API 等は組織管理者向け
- 任意のリモートセッションへ CLI/API から応答を投げる公式手段はない
- Remote Control：ローカルセッションを claude.ai / アプリから操作できるが、外部から叩ける API はない
- Channels：MCP サーバー経由で稼働中セッションにイベントを push できる（research preview）。ADR-002 により採用しない
- claude-code-action / GitLab CI/CD 連携：`@claude` メンションで動くが CI 上での新規実行で、ローカル文脈の再開にはならない
- 公式 Issue に「PR コメント等の外部イベントで元の文脈を保ってセッション再開したい」という要望あり（#31244）

## 近い OSS・事例

| 名前 | 概要 | 参考にする点 |
|---|---|---|
| the-loopy-one | webhook から issue/PR を特定し `claude -p --resume` で該当セッションを再開。work item とセッションIDのレジストリ | レジストリ設計、配信ID による重複排除 |
| syntropy | GitHub/GitLab 両対応、ポーリング既定・webhook オプション | ポーリングを既定にする判断、照合（reconciler） |
| Orcha | エージェントが issue に質問し、返信で再開 | 待ち→返答の流れ（API キー前提） |
| Cyrus | Linear/GitHub/GitLab 対応、セッションIDで再開 | ランナーの分離 |
| OpenAI Symphony | issue トラッカーを制御面にする仕様。issue ごとの作業領域、poll/dispatch/reconcile ループ | 全体の設計思想 |
| Nightly Ralph loop（個人事例） | `ready` ラベルの issue を `claude -p` で処理、1 issue = 1 PR、サブスク利用、bash + systemd | 構成がほぼ同じ。権限レベルでマージ等を拒否する運用 |
| coding-agent-control-plane | ラベル付き issue を PR 化、全ツール呼び出しを認証付き MCP ゲートウェイ経由に。OWASP Agentic Top 10 に対応付け | セキュリティ設計の比較対象 |
| ralph-orchestrator | 質問イベントでループを止め、応答かタイムアウトまで待つ | 待ち状態の扱い |
| claude-peers-mcp | 各セッションをブローカーに登録し、CLI からメッセージ注入 | 不採用の方式（参考のみ） |

## 参考リンク

- https://github.com/anthropics/claude-code/issues/35607
- https://claudeissues.com/issue/31244-feature-support-external-event-triggers-e-g-github-pr-comments-to-resume-session
- https://pypi.org/project/the-loopy-one/
- https://pkg.go.dev/github.com/andrewwormald/syntropy
- https://github.com/shipstuff/Orcha
- https://gyliu513.medium.com/openai-symphony-turning-your-issue-tracker-into-a-control-plane-for-coding-agents-b9378d523100
- https://dev.to/pvhee/the-nightly-ralph-loop-14f8
- https://github.com/stacktician/coding-agent-control-plane
- https://github.com/mikeyobrien/ralph-orchestrator
- https://code.claude.com/docs/ja/gitlab-ci-cd
