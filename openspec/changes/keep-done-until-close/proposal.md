# Proposal

## Why

`agent:done` になったissueは受信箱から消え、信頼済みのコメントを書いてもエージェントは動かない。作ったPR/MRに問題があるときや、baseとコンフリクトしたときは、doneの後もやりとりが要る。今は、人が始めるラベルを付け直すしかない（#53）。

## What Changes

- openのまま `agent:done` のissueを、closeされるまで受信箱の「完了（close待ち）」の節に出す。行番号は待ちの行に続けて振る
- doneのissueに、エージェントの最新のdoneの投稿より後で信頼済みのコメントが付けば、waitingと同じ規則で再開する。GitLabでは、doneのissueのコメント数も見る
- 受信箱の返答（CLI、API、WUI）を、waitingとdoneの両方で受け付ける
- doneで終える実行は、issueのブランチの開いたPR/MRを探し、あればそれを使い続ける
- 実装する実行のシステムプロンプトに、baseとの衝突をmergeで解消する規則を足し、許可ツールに `git merge` を足す
- ADR-040を追加する

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- `agent-run`: doneからの再開、PR/MRの使い続け、衝突の解消
- `inbox`: 完了の節、通し番号、doneのissueへの返答
- `inbox-api`: 一覧の要素の `done`、詳細の `done`、返答の確認
- `web`: 完了の節、ヘッダの件数、doneのissueの返答の欄
- `state-labels`: doneからrunningへの付け替え

## Impact

- `internal/forge`：`FindOpenChangeRequest` をインターフェースとGitHub、GitLab、偽フォージに足す
- `internal/agent`：`evaluateOnce`、`decide`、`hasReply`、`pollReplies`、`keepNotes`、`afterRun`、`finish`、許可ツール、文言
- `internal/prompt`：`System` と `ApprovedSystem` がbaseのブランチを受け取る
- `internal/inbox`、`inboxapi`、`internal/inboxserve`：状態 `done`、項目 `done_since` と `change_url`、詳細の `done`
- `web/ui`：一覧、ヘッダ、1件の表示
- `docs/DECISIONS.md`（ADR-040）、`docs/SPEC.md`、README.md
