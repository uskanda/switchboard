# Tasks

## 1. 公開のクライアント

- [x] 1.1 `inboxapi/client_test.go` に `TestDefaultSocket` を書く。環境変数の組み合わせと設置したサービスのソケットの有無で、既定の場所が決まることを確かめる
- [x] 1.2 `inboxapi/inboxapi.go` の `DefaultSocket` で、どの環境変数も無いときに、ソケットがまだ無くても `/run/switchboard/inbox.sock` を返す。1.1のテストが通ることを確かめる
- [x] 1.3 `inboxapi/client_test.go` に `TestClientFindsALaterSocket` を書く。ソケットの指定の無いクライアントが、あとから作られたソケットに届くことを確かめる
- [x] 1.4 `inboxapi/client.go` で、`Socket` が空なら要求ごとに `DefaultSocket` で場所を探す。1.3のテストが通ることを確かめる

## 2. WUI

- [x] 2.1 `web/cmd/switchboard-web/main.go` で、設定の `socket` が空なら空のままクライアントに渡す。起動のログには、要求ごとに探すことを書く。`web/internal/webconfig/webconfig.go` の説明を合わせる
- [ ] 2.2 3つのサービスを実機でまとめて再起動し、`<switchboard-web>` を再起動し直さずに一覧が出ることを確かめる

## 3. 通しの確認

- [x] 3.1 PR #32のCI（`verify`）が通ったことを確かめる
