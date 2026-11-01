# Proposal

## Why

対象のプロジェクトを足すには、`config.yaml` の `projects` を手で書き、`labels sync` を実行してcloneを用意する。そのうえで、エージェントと受信箱のサービスを再起動する必要がある。再起動にはsudoが要り（ADR-034）、スマホからは済ませられない。受信箱のWeb表示（WUI）には設定の画面が既にあるので、そこからプロジェクトを足せるようにする。

## What Changes

- WUIの設定の画面に「プロジェクト」の節を足す。この端末の担当プロジェクトの一覧と状態（クローンの有無、状態ラベルの有無）、追加の欄を、指示の節より上に並べる
- 追加の欄の入力は、`ref`（`gh:owner/repo` の形の手入力）と `plan_first`（初期値はオン）だけとする。`path` は `<inbox.project_root>/<repo名>`、`host` はこの端末、`base_branch` はフォージの既定のブランチとする
- `config.yaml` に `<inbox.project_root>` を足す。このキーが無ければ、WUIからの追加は無効になる
- 受信箱のサービス（`switchboard inbox serve`）は、プロジェクトの一覧と追加のAPIを足す。追加では、受信箱のトークンでプロジェクトの実在と既定のブランチを確かめてから、`config.yaml` に1項目を足す。コメントと並びは残し、元のファイルを1世代残す
- 同じ `ref` が既にあるとき、この端末の担当プロジェクトがまだ使っていないフォージのときは、追加を拒む
- エージェントと受信箱のサービスは、設定ファイルの変化を10秒ごとに確かめ、`projects` だけを読み直す。ほかの節の変化は再起動まで反映しない
- エージェントは、読み直しで新しく現れたプロジェクトについて、`path` が無ければ自分のトークンでcloneし、6つの状態ラベルを作る。失敗したら、そのプロジェクトは扱わず、次の照合でやり直す
- エージェントの設定に `<agent.relay>` があるとき、追加のあとにwebhookの登録のコマンド（`switchboard relay hook add <ref>`）を案内する
- 新しいADR-039で、ADR-035の「`switchboard` は設定ファイルを書き換えない」を、受信箱のサービスがプロジェクトを足す場合に限って改める

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- `web`: 設定の画面のプロジェクトの節と、追加のPOSTの検査
- `inbox-api`: プロジェクトの一覧と状態、プロジェクトの追加、設定の読み直し
- `config`: `<inbox.project_root>`、プロジェクトの追加の書き込み、設定の読み直し
- `agent-run`: 設定の読み直しと、新しいプロジェクトのcloneとラベルの作成

## Impact

- `internal/config`：`<inbox.project_root>` を足す。yaml.v3のノードでプロジェクトを1項目足す書き込みと、`projects` だけを入れ替える読み直しを足す
- `internal/inboxserve`：プロジェクトの一覧と追加のAPI、設定の読み直しを足す
- `internal/forge`：リポジトリの実在と既定のブランチ、ラベルの一覧を受信箱のトークンで読む口を足す
- `internal/agent`：設定の読み直し、新しいプロジェクトのcloneとラベルの作成を足す
- `internal/workspace`：httpsのURLからトークンを使ってcloneする口を足す
- `inboxapi`：プロジェクトの一覧と追加の型、誤りの種類を足す
- `web/internal/server`：プロジェクトの一覧と追加の口を足す
- `web/ui`：設定の画面にプロジェクトの節を足す
- `docs/DECISIONS.md` にADR-039を足す。README.mdと `config/switchboard.example.yaml` を更新する
- 土台はmainとする。`add-web-two-pane-live` は、2026-11-01にPR #35でmainへ入った
