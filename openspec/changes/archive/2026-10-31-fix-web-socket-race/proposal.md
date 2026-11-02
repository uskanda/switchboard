# Proposal

## Why

3つのサービスをまとめて再起動すると、`<switchboard-web>` が受信箱のサービスより先に起動することがある。このとき `<switchboard-web>` は、受信箱のサービスのソケットの場所を空のまま決めていた。そのあいだ `/api/*` はすべて503を返した。V31の確かめで見つけた既存の不具合である（PR #32）。

## What Changes

- `<inboxapi>` のクライアントは、ソケットの指定が無ければ、要求ごとにソケットの場所を探す
- 既定のソケットの場所は、設置したサービスのソケットがあればそれ、無ければ `RUNTIME_DIRECTORY` か `XDG_RUNTIME_DIR` の下とする。どちらの環境変数も無ければ、ソケットがまだ無くても `/run/switchboard/inbox.sock` とする
- `<switchboard-web>` は、`<web.yaml>` の `socket` が空なら、空のままクライアントに渡す
- systemdのユニットは変えない。設置し直しは要らない

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- `inbox-api`: 公開のクライアントが、指定の無いソケットの場所を探す順と時機を足す
- `web`: 受信箱のサービスより先に起動したときのつなぎを足す

## Impact

- `inboxapi/client.go`、`inboxapi/inboxapi.go`、`inboxapi/client_test.go`
- `web/cmd/switchboard-web/main.go`、`web/internal/webconfig/webconfig.go`
