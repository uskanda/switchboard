# Spec Delta

## ADDED Requirements

### Requirement: 受信箱の読み方のモードの設定
設定は、`<inbox.read_mode>` に、受信箱のサービスが一覧でissueを読む頻度の決め方を持てなければならない（MUST）。値は `adaptive` か `always` とする。
`<inbox.read_mode>` が無いとき、受信箱のサービスは `adaptive` として動かなければならない（MUST）。
`<inbox.read_mode>` が `adaptive` と `always` のどちらでもないとき、設定の検証は、設定を拒まなければならない（MUST）。

#### Scenario: 書かない
- **WHEN** `<inbox.read_mode>` の無い設定で `switchboard inbox serve` を起動する
- **THEN** 一覧は、検索の結果が変わらないissueの読む間隔を延ばす

#### Scenario: 知らない値
- **WHEN** `<inbox.read_mode>` に `lazy` を書いて設定を検証する
- **THEN** 設定は拒まれ、誤りには `adaptive` と `always` が示される

### Requirement: 受信箱の読む間隔の上限の設定
設定は、`<inbox.idle_interval>` に、`adaptive` で延ばす読む間隔の上限を持てなければならない（MUST）。
`<inbox.idle_interval>` が無いとき、上限は30分としなければならない（MUST）。
`<inbox.idle_interval>` が `<inbox.refresh_interval>` より短いとき、設定の検証は、設定を拒まなければならない（MUST）。

#### Scenario: 書かない
- **WHEN** `<inbox.idle_interval>` の無い設定で `switchboard inbox serve` を起動する
- **THEN** 検索の結果が変わらないissueは、延ばしても30分に1回は読まれる

#### Scenario: 取り直しの間隔より短い
- **WHEN** `<inbox.refresh_interval>` が `1m` で、`<inbox.idle_interval>` に `30s` を書いて設定を検証する
- **THEN** 設定は拒まれ、誤りには `<inbox.refresh_interval>` 以上にすることが示される
