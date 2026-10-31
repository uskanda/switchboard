# Proposal

## Why

Phase 3までで、エージェントはsystemdのサービスとして常駐し、GitHubとGitLabのissueを処理できるようになった。しかし、更新の検知はポーリングだけである。受信箱から返答しても、実行が始まるまで最大で `poll_interval` だけ待つ。間隔を縮めると、フォージへの問い合わせが増える。ROADMAP.mdのPhase 4の完了条件は、返答から実行の開始までの遅延がポーリングの間隔に依存しないことと、中継が止まってもポーリングで処理が続くことである。

## What Changes

- 新しく `switchboard relay serve` を足す。中継はフォージのwebhookを受け、署名を検証し、配信IDで重複を除く。そのうえで、フォージ種別、プロジェクト、issue番号、イベントの種別、配信IDだけをSSEで端末に配る。本文、コメント、タイトル、ユーザー名は保持せず、転送もしない（ADR-004）
- webhookの受け口とSSEの配信口は、別のリスナーにする。既定はどちらも `127.0.0.1` で、アドレスは設定で変えられる。SSEは、端末ごとのbearerトークンで認証する
- GitHubは `X-Hub-Signature-256` を検証する。GitLabは署名トークン（`webhook-signature`）を優先し、使えないプロジェクトに限って、設定で `X-Gitlab-Token` の比較を許す（ADR-017）
- エージェントに中継モードを足す。設定に `relay:` があれば、SSEを購読し、届いたissueを取り直す。つながっている間の照合は `reconcile_interval`（既定10分）ごとに行う。切れたら、すぐ `poll_interval` のポーリングに戻る
- `relay hook add`、`relay hook remove`、`relay token add` を足す。webhookのシークレットと端末のトークンを作ってSecret Serviceに置き、webhookを受信箱のトークンでAPIから登録する。権限が足りなければ、画面から登録する手順を案内する
- `service install --relay` で、中継を `<switchboard-relay.service>` として常駐させる。シークレットは、暗号化したクレデンシャルとして渡す
- DECISIONS.mdで、ADR-007（当面は公開しない）とADR-017（署名トークンを優先する）を決め、中継の認証とシークレットのADRを足す。READMEに、中継の設置、webhookの登録、GitLabの許可リストの設定を書く
- 実物で確かめるのはGitLabだけにする。GitHubの実物での確認は、公開の受け口を用意したときに行うので、GitHubのissueにする

## Capabilities

### New Capabilities

- `relay`: webhookの受信、署名の検証、重複の除去、IDだけのSSE配信、webhookとトークンの管理

### Modified Capabilities

- `agent-run`: 中継モードでの検知と、低頻度のポーリングでの照合
- `config`: 中継とエージェントの中継モードの設定
- `service`: 中継のサービスの設置と撤去

## Impact

- 新しく `internal/relay`（受信、検証、配信）と `internal/relayclient`（SSEの購読）を作る
- `internal/agent` に中継モードを足す。`internal/forge`（GitHub、GitLab、偽物）に、webhookの登録と削除を足す
- `internal/config` に `relay:` 節、エージェントの `relay:` と `reconcile_interval` を足す。`internal/service` に中継のunitを足す
- `cmd/switchboard` に、`relay serve`、`relay hook add`、`relay hook remove`、`relay token add` を足す
- GitLabの管理画面で、ローカルへの送信の許可リストに受け口のアドレスを足す必要がある
- README.md、DECISIONS.md、ROADMAP.md、DESIGN.md 4.7節を更新する
