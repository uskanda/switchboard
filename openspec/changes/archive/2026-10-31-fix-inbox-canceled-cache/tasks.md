# Tasks

## 1. 一覧の取得とキャッシュ

- [x] 1.1 `internal/inboxserve/cache_test.go` に、不具合を再現するテストを2つ書く。1つは、取り消した要求のあとの一覧に誤りが無く、フォージへの問い合わせが1回であることを確かめる（`TestACanceledRequestStillFetches`）。もう1つは、誤りを含む一覧のあとの要求でフォージに問い合わせ直すことを確かめる（`TestAListingWithErrorsIsNotCached`）
- [x] 1.2 `internal/inboxserve/serve.go` で、一覧を要求から切り離したcontextで取り、30秒（`FetchTimeout`）を上限にする。1.1のテストが通ることを確かめる
- [x] 1.3 `serve.go` で、届かなかったフォージがある一覧を返すがキャッシュしないようにする。1.1のテストが通ることを確かめる

## 2. 通しの確認

- [x] 2.1 PR #20のCI（`verify`）が通ったことを確かめる
