## ADDED Requirements

### Requirement: 上限の待ち時間の設定
設定は、`<agent.limit_wait>` で、解除時刻の分からない利用上限の後に待つ時間を受け付けなければならない（MUST）。書かれていないとき、値は30分とする。
`<agent.limit_wait>` が負のとき、設定の読み込みは失敗しなければならない（MUST）。

#### Scenario: 待ち時間を書かない
- **WHEN** `<agent.limit_wait>` の無い設定を読む
- **THEN** 待ち時間は30分になる

#### Scenario: 負の待ち時間
- **WHEN** `<agent.limit_wait>` に `-1m` を書いた設定を読む
- **THEN** 読み込みは `<agent.limit_wait>` を名指す誤りで失敗する

### Requirement: クオータの設定
設定は、最上位の `quota` 節で、`<quota.daily_usd>` と `<quota.monthly_usd>` を受け付けなければならない（MUST）。書かれていない項目のクオータは無効とする。
どちらかが負のとき、設定の読み込みは失敗しなければならない（MUST）。
`<claude.auth>` が `subscription` で `quota` が書かれているとき、`switchboard doctor` は、クオータを使わないという警告を出さなければならない（MUST）。

#### Scenario: 日のクオータを書く
- **WHEN** `quota: {daily_usd: 5}` と書いた設定を読む
- **THEN** 日のクオータは5ドルになり、月のクオータは無効のままである

#### Scenario: 負のクオータ
- **WHEN** `<quota.monthly_usd>` に-1を書いた設定を読む
- **THEN** 読み込みは `<quota.monthly_usd>` を名指す誤りで失敗する

#### Scenario: サブスクリプションでクオータを書く
- **WHEN** `<claude.auth>` が `subscription` で、`<quota.daily_usd>` を書いた設定で `switchboard doctor` を実行する
- **THEN** クオータを使わないという警告が出る。失敗の数には数えない
