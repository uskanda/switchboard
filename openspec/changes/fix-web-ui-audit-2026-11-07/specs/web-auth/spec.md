## ADDED Requirements

### Requirement: ログインの状態が読めなかったときのセッション切れ
`/api/auth` が読めずにログインを無効と見なしていても、Web表示は、401 `unauthenticated` を受けたらログインの画面を出さなければならない（MUST）。401 `unauthenticated` は、ログインが有効であることを示すからである。

#### Scenario: 受信箱のサービスの再起動中に開く
- **WHEN** `/api/auth` が503を返し、続く `/api/inbox` が401 `unauthenticated` を返す
- **THEN** ログインの画面が出る

### Requirement: ログアウトの失敗
ログアウトのPOSTが成功の応答を返さなかったとき、Web表示は、ログインした状態を保ち、失敗をトーストで知らせなければならない（MUST）。ログインの画面を出してはならない（MUST NOT）。

#### Scenario: 受信箱のサービスが止まっている
- **WHEN** ログアウトのPOSTが503を返す
- **THEN** ヘッダにはログインした人が出たままで、ログアウトに失敗したことがトーストで出る
