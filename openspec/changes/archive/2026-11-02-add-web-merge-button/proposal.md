# Proposal

## Why

完了（close待ち）のissueを片付けるたびに、マスターはフォージでPRを開き、draftを解除してマージしていた。スマホではこの操作が手間である（#69）。受信箱のWeb表示（WUI）から、画面で見たコミットに限ってマージできるようにする。

## What Changes

- 受信箱の一覧の要素と1件の詳細に、完了のissueのPR/MRの番号 `change_number` とコミット `change_sha` を足す。どちらもエージェントの最新のdoneの投稿のマーカーから取る
- 受信箱のサービスに `POST /v1/merge` を足す。openで `agent:done` のissueについて、マーカーが名指すPR/MRを、受信箱のトークンでdraftから解除してマージする。マージは、ブラウザが送った `sha` とマーカーの `sha` が一致し、PR/MRのheadもその `sha` のときに限る
- WUIのサーバーに `POST /api/merge` を足し、返答と同じ `Origin` と `Host` の検査を通ったものだけを受信箱のサービスに渡す
- WUIの一覧の完了の行と1件の表示に「マージ」のボタンを置く。押すと確認のダイアログを開き、結果をトーストで知らせる
- 受信箱のサービスは、issueを直接closeせず、ラベルとコメントにも触らない。closeはPR/MRの `Closes #N` に任せる
- 受信箱のトークンの権限を広げる。GitHubでは `Pull requests: write` と `Contents: write` を足す。ADR-043を追加し、README、設定例、SECURITY.md、ARCHITECTURE.md、doctorの説明を直す
- ROADMAPの検証項目にV38〜V40を「未確認」として足す

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- `inbox-api`: 完了の項目のPR/MRの番号とコミット、マージの受け付け、マージの前の確認、マージの誤り
- `web`: マージのボタン、確認のダイアログ、結果のトースト、マージのPOSTの検査

## Impact

- `internal/forge`：インターフェース `Merges` と誤り `ErrHeadMoved`、`NotMergeableError` を足す。GitHub、GitLab、偽フォージに実装する
- `internal/inbox`：`Item` と `Detail` の項目、`Inbox.Merge`
- `inboxapi`：`MergeRequest`、クライアントの `Merge`、誤りのコード `stale`、`not_mergeable`、`forbidden`
- `internal/inboxserve`：`/v1/merge` の道
- `cmd/switchboard`：受信箱のトークンのクライアントを `Merges` として渡す
- `web/internal/server`：`/api/merge`
- `web/ui`：`MergeButton`、`components/ui/dialog.tsx`、`InboxList`、`IssuePane`、`lib/api.ts`
- `internal/doctor`：受信箱のトークンについて、マージに要る権限を表示する
- 文書：`docs/DECISIONS.md`（ADR-043）、`docs/SECURITY.md`、`docs/ARCHITECTURE.md`、`docs/DESIGN.md`、`docs/ROADMAP.md`
- そのほかの文書：DESIGN.md、README.md、`config/switchboard.example.yaml`
