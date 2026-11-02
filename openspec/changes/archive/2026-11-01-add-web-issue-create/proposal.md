# Proposal

## Why

スマホから作業を頼むには、フォージの画面でissueを立て、開始ラベルを付ける必要がある。受信箱のWeb表示（WUI）は既にスマホから使えるので、そこから設定のプロジェクトを選んでissueを作り、そのままエージェントに着手させたい（issue #40）。

## What Changes

- WUIの一覧のペインの上に「新規issue」のボタンを置き、ダイアログでissueを作る。項目は、プロジェクト、タイトル（必須）、本文（Markdown、任意）、開始ラベルの4つである
- 選べるプロジェクトは、設定にあるすべてのプロジェクトとする。選択肢には担当の端末の名前を出す
- 開始ラベルは、ready、plan、goから1つを選ぶ。初期値はreadyとする。ブラウザは種別だけを送り、ラベル名は核が設定から決める
- 作ったあとはダイアログを閉じ、一覧の上に、作ったissueとフォージへのリンクを知らせとして出す
- 受信箱のサービス（`switchboard inbox serve`）に、作れる先の一覧（`GET /v1/issue-targets`）と、issueの作成（`POST /v1/issues`）のAPIを足す。issueは受信箱のトークンで作る
- 新しいADR-040で、認証の無いWUIから信頼済みのissueを作って実行を起こせることを、受け入れたリスクとして記録する

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- `inbox-api`: 作れる先の一覧と、issueの作成
- `web`: 新規issueのダイアログ、作成の結果の表示、作成のPOSTの検査

## Impact

- `internal/forge`：issueを作る口 `Issues` を足し、GitHub、GitLab、偽のフォージで実装する
- `internal/inboxserve`：`/v1/issue-targets` と `/v1/issues` を足す
- `inboxapi`：型とクライアントの `IssueTargets`、`CreateIssue` を足す
- `cmd/switchboard`：受信箱のトークンのクライアントを `Issues` として渡す
- `web/internal/server`：`/api/issue-targets` と `/api/issues` を足す
- `web/ui`：`NewIssue` の部品を一覧の上に足す
- `docs/DECISIONS.md` にADR-040を足す。`docs/SECURITY.md` のT5、`docs/SPEC.md` のF4、`DESIGN.md` の部品を更新する
- `docs/ROADMAP.md` に検証項目V33とV34を足す
- 受信箱のトークンの権限は広げない（Issuesの読み書きで足りる）
