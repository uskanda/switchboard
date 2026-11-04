# Spec Delta

## MODIFIED Requirements

### Requirement: 中継の設定
設定の `<relay>` 節は、中継のwebhookの受け口とSSEの配信口のアドレス、webhookの一覧、端末のトークンの一覧を持つ。
webhookの一覧の各項目は、プロジェクトの `ref`、`hook-key`、シークレットの `secret_ref` を持つ。GitLabのプロジェクトでは、`<gitlab_token>` を持ってよい（MAY）。
端末のトークンの一覧の各項目は、端末名とトークンのハッシュを持つ。
設定の検証は、`<relay>` 節にシークレットやトークンの値と見える文字列があれば、設定の誤りとして拒否しなければならない（MUST）。ハッシュは、決まった接頭辞で値と見分ける。
設定の検証は、`<relay.webhook_listen>` がIPアドレスとポートの形でないとき、ホスト名のとき、すべてのインターフェース（`0.0.0.0`、`::`）のときに、設定の誤りとして拒否しなければならない（MUST）。
`<relay.webhook_listen>` がループバックでない間、設定の検証は、`<gitlab_token>: true` の項目を設定の誤りとして拒否しなければならない（MUST）。

#### Scenario: hook-keyの重複
- **WHEN** `<relay>` 節の2つのwebhookが同じ `hook-key` を持つ
- **THEN** 設定の検証は誤りを報告し、`relay serve` は起動しない

#### Scenario: ホスト名の受け口
- **WHEN** `<relay.webhook_listen>` が `localhost:8781` である
- **THEN** 設定の検証は、IPアドレスで書くよう示す誤りを報告する

#### Scenario: 既定の受け口
- **WHEN** `<relay.webhook_listen>` を書いていない
- **THEN** 設定は通り、受け口は `127.0.0.1:8781` になる

#### Scenario: LANの受け口でのgitlab_token
- **WHEN** `<relay.webhook_listen>` が `192.168.0.82:8781` で、`relay.hooks[0]` に `<gitlab_token>: true` がある
- **THEN** 設定の検証は `relay.hooks[0].gitlab_token` の誤りとして、署名トークンを使うよう示す
