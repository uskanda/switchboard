# Proposal

## Why

WUIでマージを押し、PR/MRがbaseと衝突していて断られると、エージェントが衝突を直して新しいdoneを投稿する（ADR-044）。そのあと、人がもう一度画面を開いてマージを押し直す必要があった（issue #119）。一度マージを指示したのだから、衝突が直ったら極力自動でマージしたい。

## What Changes

- 受信箱のサービスは、衝突で断られたマージを「マージの依頼」として状態ディレクトリのファイルに覚え、`POST /v1/merge` に202と `auto_merge_pending` で答える
- 受信箱のサービスは、依頼があるあいだ `inbox.refresh_interval` ごとに（マージの受付と中継のイベントでも）依頼を調べる。エージェントの直す実行が新しいdoneを投稿したら、そのマーカーの `sha` を指定して受信箱のトークンでマージする。画面がつながっていなくても調べる
- 押したあとに人の指示の実行が入ったとき、issueのclose、`waiting` への移行、PR/MRのcloseとマージ、人のpushでは依頼を打ち切る
- doneのマーカーに、直す実行の投稿であることを示す `fix=<ci|push|conflict>` を足す
- APIの一覧と詳細に `auto_merge`、一覧に終わった依頼の `auto_merge_results` を足す。`POST /v1/automerge/cancel` で依頼を取り消せる
- WUIは、202をトーストで知らせ、一覧の完了の行と詳細のペインに「自動マージ待ち」の印を付け、詳細のペインに取り消しのボタンを置く。終わった依頼の結果をトーストで出す
- ADR-048を足し、ADR-043の「画面で見た `sha` に限る」を、押したあとに直す実行がpushしたコミットまで広げる

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- `run-outcome`：doneのマーカーに `fix` を足す
- `inbox-api`：マージの応答に202を足し、自動マージの依頼、見張り、取り消し、一覧と詳細の項目を足す
- `web`：マージの結果の表示に202を足し、自動マージ待ちの印、取り消し、結果のトーストを足す

## Impact

- Goのエージェント側：`internal/marker`、`internal/agent`（直す実行のdoneの書き戻し）
- Goのフォージの層：`internal/forge`（`Merges` に `ChangeRequestHead` を足す）、`internal/forge/fake`
- Goの受信箱側：新しい `internal/automerge`、`internal/inbox`、`internal/inboxserve`、`cmd/switchboard`
- GoのAPI：`inboxapi`（クライアントのマージの戻り値が変わる）、`web/internal/server`
- Web UI：`MergeButton`、`InboxList`、`IssuePane`、`lib/api.ts`、受信箱の購読
- 文書：`docs/DECISIONS.md`（ADR-048）、`docs/ARCHITECTURE.md`、`docs/SPEC.md`、`docs/ROADMAP.md`、`README.md`、`DESIGN.md`
- トークンの権限は変えない
