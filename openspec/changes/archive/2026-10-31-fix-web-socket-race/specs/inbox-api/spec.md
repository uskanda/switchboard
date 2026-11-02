## ADDED Requirements

### Requirement: 公開のクライアントのソケットの探し方
ソケットの場所を指定されていない間、`<inboxapi>` のクライアントは、要求ごとにソケットの場所を探さなければならない（MUST）。

#### Scenario: クライアントのあとに受信箱のサービスが起動する
- **WHEN** ソケットの指定の無いクライアントが一覧を求めて届かず、そのあとで `switchboard inbox serve` が既定の場所にソケットを作り、クライアントがもう一度一覧を求める
- **THEN** 1回目は受信箱のサービスに届かない誤りが返り、2回目はソケットを通して一覧が返る

### Requirement: 既定のソケットの場所
`<inboxapi>` のクライアントは、ソケットの場所を指定されていないとき、次の順に既定の場所を決めなければならない（MUST）。

1. `/run/switchboard/inbox.sock` があれば、その場所
2. `RUNTIME_DIRECTORY` があれば、その下の `inbox.sock`
3. `XDG_RUNTIME_DIR` があれば、`$XDG_RUNTIME_DIR/switchboard/inbox.sock`
4. どれにも当たらなければ、ソケットがまだ無くても `/run/switchboard/inbox.sock`

#### Scenario: systemdから環境変数無しで起動する
- **WHEN** `RUNTIME_DIRECTORY` と `XDG_RUNTIME_DIR` のどちらも無く、`/run/switchboard/inbox.sock` もまだ無いときに、既定の場所を求める
- **THEN** `/run/switchboard/inbox.sock` が返る

#### Scenario: 設置したサービスのソケットがある
- **WHEN** `/run/switchboard/inbox.sock` があり、`XDG_RUNTIME_DIR` もあるときに、既定の場所を求める
- **THEN** `/run/switchboard/inbox.sock` が返る
