# Spec Delta

## ADDED Requirements

### Requirement: プッシュ通知の設定
設定は、`<inbox.notify.fcm>` に、`project_id` と `credentials_ref` を持てなければならない（MUST）。前者はFirebaseのプロジェクトのID、後者はサービスアカウントの鍵の秘密の名前である。
`<inbox.notify.fcm>` を書いて、`project_id` か `credentials_ref` が無いとき、設定の検証は、設定を拒まなければならない（MUST）。
`<inbox.notify.fcm>` が無い設定は、今までどおり読めなければならない（MUST）。そのとき、受信箱のサービスはプッシュ通知を送らない。
`<inbox.notify.fcm>` の変更は、受信箱のサービスを再起動したときに反映しなければならない（MUST）。

#### Scenario: 鍵の名前が無い
- **WHEN** `<inbox.notify.fcm>` に `project_id` だけを書いて設定を検証する
- **THEN** 設定は拒まれ、誤りには `credentials_ref` が示される

#### Scenario: 書かない
- **WHEN** `<inbox.notify>` の無い設定で `switchboard inbox serve` を起動する
- **THEN** 誤りにならず、プッシュ通知は送られない

## MODIFIED Requirements

### Requirement: 受信箱の更新の間隔の設定
`<inbox.refresh_interval>` は、受信箱のサービスが、画面を開いている間にフォージから一覧を取り直す間隔でなければならない（MUST）。
`<inbox.notify.fcm>` があるとき、`<inbox.refresh_interval>` は、画面を開いていなくても一覧を取り直す間隔でなければならない（MUST）。
`<inbox.refresh_interval>` が無いとき、間隔は30秒としなければならない（MUST）。
`<inbox.refresh_interval>` が10秒より短いとき、設定の検証は、設定を拒まなければならない（MUST）。

#### Scenario: 書かない
- **WHEN** `inbox` の節が無い設定で `switchboard inbox serve` を起動する
- **THEN** 一覧は30秒ごとに取り直される

#### Scenario: 短すぎる
- **WHEN** `<inbox.refresh_interval>` に `5s` を書いて設定を検証する
- **THEN** 設定は拒まれ、誤りには下限の10秒が示される
