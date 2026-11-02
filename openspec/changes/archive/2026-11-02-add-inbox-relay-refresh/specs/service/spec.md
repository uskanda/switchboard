## MODIFIED Requirements

### Requirement: 受信箱のサービス
`service install --inbox` は、`switchboard inbox serve` をsystemdのサービスとして設置し、有効にして起動しなければならない（MUST）。unitの名前は `<switchboard-inbox.service>` とする。unitには `RuntimeDirectory=switchboard` を指定し、ソケットを `/run/switchboard/inbox.sock` に置く。
`service install --inbox` は、設定の各フォージの `inbox_token_ref` を、ホストの鍵で暗号化したsystemdのクレデンシャルにしなければならない（MUST）。`<inbox.relay>` があれば、その `token_ref` もクレデンシャルにする。エージェント用のトークンをクレデンシャルにしてはならない（MUST NOT）。
`service uninstall --inbox` と `service status --inbox` は、このサービスについて、エージェントのサービスと同じことをする。

#### Scenario: 受信箱のサービスを設置する
- **WHEN** GitHubとGitLabの設定で `service install --inbox` を実行し、確認に `y` と答える
- **THEN** 2つの `inbox_token_ref` だけがクレデンシャルになり、サービスが動き出してソケットができる

#### Scenario: 中継につなぐ受信箱のサービスを設置する
- **WHEN** `<inbox.relay>` のある設定で `service install --inbox` を実行する
- **THEN** 各フォージの `inbox_token_ref` に加えて、`<inbox.relay>` の `token_ref` がクレデンシャルになる
