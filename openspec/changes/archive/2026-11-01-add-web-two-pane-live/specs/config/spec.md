## ADDED Requirements

### Requirement: 受信箱の更新の間隔の設定
`<inbox.refresh_interval>` は、受信箱のサービスが、画面を開いている間にフォージから一覧を取り直す間隔でなければならない（MUST）。
`<inbox.refresh_interval>` が無いとき、間隔は30秒としなければならない（MUST）。
`<inbox.refresh_interval>` が10秒より短いとき、設定の検証は、設定を拒まなければならない（MUST）。

#### Scenario: 書かない
- **WHEN** `inbox` の節が無い設定で `switchboard inbox serve` を起動する
- **THEN** 一覧は30秒ごとに取り直される

#### Scenario: 短すぎる
- **WHEN** `<inbox.refresh_interval>` に `5s` を書いて設定を検証する
- **THEN** 設定は拒まれ、誤りには下限の10秒が示される
