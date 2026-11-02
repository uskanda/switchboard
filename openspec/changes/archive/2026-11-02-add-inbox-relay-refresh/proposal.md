# Proposal

## Why

受信箱のWeb表示の自動更新は、`<inbox.refresh_interval>`（既定30秒）ごとにフォージへ問い合わせる。そのため、変化が画面に出るまで最大で間隔の分だけ遅れる。中継は既にwebhookのイベントをエージェントへ配っているが、受信箱のサービスはそれを受けていない。

## What Changes

- `switchboard relay token add --inbox <端末名>` を足す。受信箱のサービス用に、エージェントとは別のトークンを作る
- 設定に `<inbox.relay>`（`url` と `token_ref`）を足す。あれば、`switchboard inbox serve` は動いている間ずっと中継の配信口につなぐ
- 中継のイベントが届いたら、画面がつないでいる間は2秒まとめてから一覧を取り直す。前回の取得から10秒たつまでは取り直さない。画面が無いときは、一覧のキャッシュを捨てる
- `<inbox.refresh_interval>` ごとの問い合わせは、中継につないでいても延ばさない
- `inboxapi.Inbox` に、中継の状態（`relay`）と問い合わせの間隔（`refresh_interval_seconds`）を足す
- Web表示のヘッダの文言を、中継の状態で「自動更新中（即時）」「自動更新中（N秒ごと）」「自動更新中」に分ける
- `service install --inbox` は、`<inbox.relay>` のトークンもクレデンシャルとして渡す

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

- `relay`：端末のトークンに、受信箱のサービス用の発行を足す
- `config`：`<inbox.relay>` の項目を足す
- `inbox-api`：中継のイベントによる取り直しと、一覧の `relay` と `refresh_interval_seconds` を足す
- `web`：ヘッダの自動更新の文言を中継の状態で分ける
- `service`：受信箱のサービスのクレデンシャルに、中継のトークンを足す

## Impact

- `internal/config`、`internal/relay`、`internal/service`、`internal/inboxserve`、`inboxapi`、`cmd/switchboard`
- `web/ui` のヘッダと型
- `README.md`、`DESIGN.md`、`docs/DECISIONS.md` のADR-035への追記
