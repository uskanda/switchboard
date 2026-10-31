## ADDED Requirements

### Requirement: 受信箱のサービス
`service install --inbox` は、`switchboard inbox serve` をsystemdのサービスとして設置し、有効にして起動しなければならない（MUST）。unitの名前は `<switchboard-inbox.service>` とする。unitには `RuntimeDirectory=switchboard` を指定し、ソケットを `/run/switchboard/inbox.sock` に置く。
`service install --inbox` は、設定の各フォージの `inbox_token_ref` だけを、ホストの鍵で暗号化したsystemdのクレデンシャルにしなければならない（MUST）。エージェント用のトークンをクレデンシャルにしてはならない（MUST NOT）。
`service uninstall --inbox` と `service status --inbox` は、このサービスについて、エージェントのサービスと同じことをする。

#### Scenario: 受信箱のサービスを設置する
- **WHEN** GitHubとGitLabの設定で `service install --inbox` を実行し、確認に `y` と答える
- **THEN** 2つの `inbox_token_ref` だけがクレデンシャルになり、サービスが動き出してソケットができる

### Requirement: WUIのサービス
`service install --web` は、`<switchboard-web>` をsystemdのサービスとして設置し、有効にして起動しなければならない（MUST）。unitの名前は `<switchboard-web.service>` とする。`<switchboard-web>` の場所は、設置のときに `PATH` から探して絶対パスで書く。見つからなければ、何もせずに理由を報告する。
WUIのサービスには、クレデンシャルを渡してはならない（MUST NOT）。
`service uninstall --web` と `service status --web` は、このサービスについて、エージェントのサービスと同じことをする。

#### Scenario: switchboard-webが無い
- **WHEN** `PATH` に `<switchboard-web>` が無い端末で `service install --web` を実行する
- **THEN** sudoは呼ばれず、`<switchboard-web>` が見つからないと報告される

## MODIFIED Requirements

### Requirement: クレデンシャル
`service install` は、この端末の担当プロジェクトのエージェント用トークンだけを、systemdのクレデンシャルにしなければならない（MUST）。対象は、フォージの `token_ref` と、プロジェクトの上書きの `token_ref` である。設定の `agent` に `<relay>` があれば、その `token_ref`（中継のbearerトークン）も対象にする。設定に `<notify.telegram>` があれば、その `token_ref`（Telegramのbotのトークン）も対象にする。クレデンシャルは、ホストの鍵で暗号化する。
`service install` は、受信箱のトークン（`inbox_token_ref`）をクレデンシャルにしてはならない（MUST NOT）。
トークンの値を、コマンドの引数、表示、平文のファイルに出してはならない（MUST NOT）。

#### Scenario: 置くクレデンシャル
- **WHEN** GitHubとGitLabのプロジェクトを1つずつ担当する端末で `service install` を実行する
- **THEN** `token_ref` の2つだけがクレデンシャルになり、`inbox_token_ref` はならない

#### Scenario: 中継モードのエージェント
- **WHEN** `agent` に `<relay>` がある設定で `service install` を実行する
- **THEN** 中継のbearerトークンもクレデンシャルになる

#### Scenario: 通知するエージェント
- **WHEN** `<notify.telegram>` がある設定で `service install` を実行する
- **THEN** Telegramのbotのトークンもクレデンシャルになり、`inbox_token_ref` はならない
