## ADDED Requirements

### Requirement: 受信箱の中継の設定
設定は、`<inbox.relay>` に、中継の配信口のURL（`url`）と、受信箱のサービスのトークンの `token_ref` を持てなければならない（MUST）。無いとき、受信箱のサービスは中継につながない。
`<inbox.relay>` を書いて `url` か `token_ref` が無いとき、設定の検証は、設定を拒まなければならない（MUST）。

#### Scenario: token_refが無い
- **WHEN** `<inbox.relay>` に `url` だけを書いて設定を検証する
- **THEN** 設定は拒まれ、誤りには `inbox.relay.token_ref` が示される
