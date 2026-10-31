## ADDED Requirements

### Requirement: 中継のサービス
`service install --relay` は、中継をsystemdのサービス `<switchboard-relay.service>` として設置しなければならない（MUST）。設置したら、有効にして起動する。サービスは `switchboard relay serve` を実行し、異常に終わったら自動で再起動する。
`service install --relay` は、`<relay>` 節のwebhookのシークレットを、ホストの鍵で暗号化したsystemdのクレデンシャルにしなければならない（MUST）。フォージのトークンをクレデンシャルにしてはならない（MUST NOT）。
`service uninstall --relay` は、中継のサービス、unit、クレデンシャルを消さなければならない（MUST）。
sudoで実行するコマンドは、エージェントのサービスと同じく、全部表示して確認を取ってから実行する。

#### Scenario: 中継を設置する
- **WHEN** webhookを1つ登録した設定で `service install --relay` を実行し、確認に `y` と答える
- **THEN** そのwebhookのシークレットだけがクレデンシャルになり、中継のサービスが動き出す

## MODIFIED Requirements

### Requirement: クレデンシャル
`service install` は、この端末の担当プロジェクトのエージェント用トークンだけを、systemdのクレデンシャルにしなければならない（MUST）。対象は、フォージの `token_ref` と、プロジェクトの上書きの `token_ref` である。設定の `agent` に `<relay>` があれば、その `token_ref`（中継のbearerトークン）も対象にする。クレデンシャルは、ホストの鍵で暗号化する。
`service install` は、受信箱のトークン（`inbox_token_ref`）をクレデンシャルにしてはならない（MUST NOT）。
トークンの値を、コマンドの引数、表示、平文のファイルに出してはならない（MUST NOT）。

#### Scenario: 置くクレデンシャル
- **WHEN** GitHubとGitLabのプロジェクトを1つずつ担当する端末で `service install` を実行する
- **THEN** `token_ref` の2つだけがクレデンシャルになり、`inbox_token_ref` はならない

#### Scenario: 中継モードのエージェント
- **WHEN** `agent` に `<relay>` がある設定で `service install` を実行する
- **THEN** 中継のbearerトークンもクレデンシャルになる
