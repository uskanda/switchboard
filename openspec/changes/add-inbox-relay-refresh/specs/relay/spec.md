## MODIFIED Requirements

### Requirement: 端末のトークン
`switchboard relay token add <host>` は、端末のbearerトークンを作り、その値をSecret Serviceに置かなければならない（MUST）。そのうえで、設定の `<relay>` 節に足す項目（端末名とトークンのハッシュ）を表示する。
`switchboard relay token add --inbox <host>` は、その端末の受信箱のサービス用に、エージェントとは別のトークンを作らなければならない（MUST）。端末名は `<host>-inbox`、置き場所は `switchboard/relay-<host>-inbox` とし、`<inbox.relay>` に足す項目を表示する。
トークンの値を、設定ファイル、ログ、コマンドの引数に出してはならない（MUST NOT）。

#### Scenario: 発行する
- **WHEN** `relay token add usknas` を実行する
- **THEN** トークンの値はSecret Serviceに置かれ、設定に足す項目としてハッシュだけが表示される

#### Scenario: 受信箱のサービス用に発行する
- **WHEN** `relay token add --inbox usknas` を実行する
- **THEN** トークンは `switchboard/relay-usknas-inbox` に置かれ、エージェントのトークンは変わらない。`relay.hosts` に足す `usknas-inbox` の項目と、`inbox.relay` に足す項目が表示される
