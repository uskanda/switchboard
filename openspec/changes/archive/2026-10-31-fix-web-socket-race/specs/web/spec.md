## ADDED Requirements

### Requirement: 受信箱のサービスより先に起動したときのつなぎ
`<web.yaml>` の `socket` が空のとき、`<switchboard-web>` は、受信箱のサービスのソケットの場所を、起動時でなく要求ごとに探さなければならない（MUST）。

#### Scenario: まとめて再起動する
- **WHEN** 3つのサービスをまとめて再起動し、`<switchboard-web>` が `switchboard inbox serve` より先に起動する
- **THEN** 受信箱のサービスがソケットを作ったあとは、`<switchboard-web>` を再起動しなくても、一覧の画面に一覧が出る
