# Tasks

## 1. テスト

- [x] 1.1 `web/internal/server/server_test.go` に、gzip、`Cache-Control`、`ETag` と304、HEAD、キャッシュの使い回しのテストを足し、失敗を確かめた

## 2. 実装

- [x] 2.1 `web/internal/server/server.go` に `staticFile` とファイルごとのキャッシュを足し、`static` を `http.ServeContent` で返すように直す
- [x] 2.2 1.1のテストが通ることを確かめた

## 3. 通しの確認

- [x] 3.1 `make verify` が通ることを確かめた
- [x] 3.2 `Server` に、ビルドした実物の資産を渡し、`/` と `assets/` のファイルの見出し、gzipでの転送量、304を確かめた。JSの合計は547KBで、gzipでは170KBになった。このrunではサーバーを起動できなかったため、`curl` の代わりに一時のGoのテストで要求を送った
