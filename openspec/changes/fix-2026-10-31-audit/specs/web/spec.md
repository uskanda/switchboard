# Spec Delta

## ADDED Requirements

### Requirement: listenはLANのアドレスに限る
`switchboard-web` は、`listen` のアドレスが、ループバック、プライベート（RFC 1918、`fc00::/7`）、リンクローカルのどれでもないとき、起動してはならない（MUST NOT）。IPv4射影のIPv6アドレスは、IPv4に戻してから検査しなければならない（MUST）。

#### Scenario: IPv4射影の全インターフェース
- **WHEN** `listen` が `[::ffff:0.0.0.0]:8783` である
- **THEN** `switchboard-web` は設定の誤りとして起動しない

#### Scenario: CGNATとグローバルのアドレス
- **WHEN** `listen` が `100.64.0.1:8783` か、グローバルなアドレスである
- **THEN** `switchboard-web` は設定の誤りとして起動しない

#### Scenario: プライベートのアドレス
- **WHEN** `listen` が `192.168.1.10:8783` である
- **THEN** `switchboard-web` は起動する

### Requirement: すべての要求でHostを確かめる
要求の `Host` が `listen` と一致しないとき、`switchboard-web` は、GETも含めて、その要求に421で答えなければならない（MUST）。

#### Scenario: DNS rebindingのGET
- **WHEN** `Host: evil.example` の `GET /api/inbox` が届く
- **THEN** `switchboard-web` は421で断り、一覧を返さない
