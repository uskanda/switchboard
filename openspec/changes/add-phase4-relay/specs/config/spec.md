## ADDED Requirements

### Requirement: 中継の設定
設定の `<relay>` 節は、中継のwebhookの受け口とSSEの配信口のアドレス、webhookの一覧、端末のトークンの一覧を持つ。
webhookの一覧の各項目は、プロジェクトの `ref`、`hook-key`、シークレットの `secret_ref` を持つ。GitLabのプロジェクトでは、`<gitlab_token>` を持ってよい（MAY）。
端末のトークンの一覧の各項目は、端末名とトークンのハッシュを持つ。
設定の検証は、`<relay>` 節にシークレットやトークンの値と見える文字列があれば、設定の誤りとして拒否しなければならない（MUST）。ハッシュは、決まった接頭辞で値と見分ける。

#### Scenario: hook-keyの重複
- **WHEN** `<relay>` 節の2つのwebhookが同じ `hook-key` を持つ
- **THEN** 設定の検証は誤りを報告し、`relay serve` は起動しない

### Requirement: エージェントの中継モードの設定
設定の `agent` は、`<relay>`（中継のSSEのURLと、bearerトークンの `token_ref`）と `<reconcile_interval>` を持ってよい（MAY）。
`<relay>` が無いとき、エージェントは今までどおり `poll_interval` ごとのポーリングだけで動かなければならない（MUST）。
`<reconcile_interval>` は `poll_interval` 以上でなければならない（MUST）。

#### Scenario: 中継の設定が無い
- **WHEN** `agent` に `<relay>` が無い設定で、`switchboard agent run` を実行する
- **THEN** エージェントは中継につながず、`poll_interval` ごとにポーリングする
