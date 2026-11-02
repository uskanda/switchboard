# Proposal

## Why

エージェントは、変更要求（PR/MR）を作った時点で `agent:done` にし、その後のCIを見ない。CIが失敗しても、人が気づいてログを読み、issueへ返信して直させるまで、変更要求は赤いまま残る。
pushが失敗したときも、エージェントはgitのエラーを1行書いて `waiting(reason=error)` にするだけである。リモートの枝が先に進んだだけなのか、トークンの権限が足りないのかを、人が見分けなければならない。
先行する変更 `reuse-open-change-request` で、同じブランチへの再実行が既存の変更要求を使えるようになる。失敗を直した実行のpushは、既存の変更要求に新しいコミットとして載る。

## What Changes

- 変更要求を作って `agent:done` にした後、エージェントは、自分がpushしたコミットのCIを `poll_interval` ごとに見張る。CIの結果は、GitHubとGitLabのどちらもポーリングで知る。中継は変えない
- CIが失敗したら、エージェントは、失敗したjobのログの末尾を取得し、非信頼のデータとしてプロンプトに入れる。そのうえで、issueを `agent:running` に戻して直す実行を始める。直した結果は同じ変更要求にpushし、もう一度見張る
- pushの失敗を、一時的な失敗、non-fast-forward、権限・認証、リモートの規則による拒否の4つに分ける。一時的な失敗はやり直す。non-fast-forwardは、リモートの枝をfetchしてClaudeにmergeさせる。権限と規則による拒否は、`waiting(reason=push)` にして、人がすべきことを書く
- 自動で直す回数を、1つのissueで合わせて `<agent.max_fix_attempts>`（既定2）に限る。上限まで直しても失敗したら、`waiting(reason=ci)` にする。信頼済みの人のコメントか始めるラベルで、回数を0に戻す
- 完了のコメントのマーカーに、pushしたコミット（`sha`）と変更要求の番号（`cr`）を足す。再起動の後も、担当の端末が見張りを続けられる
- GitHubのエージェントのトークンに、`Actions: Read` と `Commit statuses: Read` を足す。`switchboard doctor` で不足を知らせる。ADR-041を足し、SECURITY.mdの4章を書き換える

## Capabilities

### New Capabilities

- `ci-watch`：エージェントが自分でpushしたコミットのCIを見張り、失敗を判定し、失敗の情報を集め、直す実行を始めるまで

### Modified Capabilities

- `run-outcome`：pushの失敗を分類して扱う。待ちの理由に `push` と `ci` を足す。完了のマーカーに `sha` と `cr` を足す。直す実行の終わりの扱いと、自動で直す回数の上限を足す
- `agent-run`：`agent:done` のissueでも、CIの失敗では直す実行を始める。直す実行のプロンプトの内容と、許すgitの操作を足す
- `config`：`<agent.max_fix_attempts>` を足す
- `web`：待ちの理由の `push` と `ci` を、ほかの理由と同じく表示し、1件の画面で人がすべきことを示す
- `notify`：待ちの通知の対象の理由に `push` と `ci` を足す

## Impact

- `internal/forge`：コミットのCIの状態（check run、commit status、pipeline）と、失敗したjobのログを読む操作を足す。GitHub、GitLab、fakeの実装
- `internal/workspace`：`Push` のstderrを分類して返す。non-fast-forwardのときにリモートの枝をfetchする
- `internal/agent`：見張りの一覧と、直す実行の種類を足す。`finish` でpushの失敗を分けて扱う
- `internal/marker`：`sha`、`cr` の項目と、`reason` の `fix`、`ci`、`push` を足す
- `internal/prompt`：CIの失敗とpushの失敗を、非信頼のデータとして入れる節
- `internal/config`、`internal/doctor`、`web/ui`（理由の表示）
- `docs/DECISIONS.md`（ADR-041）、`docs/SECURITY.md`、`docs/ROADMAP.md`（Checks APIの検証項目）
- 既存の利用者は、GitHubのトークンに2つの権限を足す必要がある。足さないうちは、エージェントはCIを見張らず、`switchboard doctor` が警告する
- `reuse-open-change-request` を先に入れることが前提
