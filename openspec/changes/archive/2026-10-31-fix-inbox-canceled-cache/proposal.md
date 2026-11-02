# Proposal

## Why

WUIの一覧に「githubを読めませんでした：… context canceled」が出続けた（PR #20）。受信箱のサービスは、一覧をブラウザの要求のcontextで取っていた。ブラウザが要求をやめると、フォージへの問い合わせも打ち切られる。打ち切りの誤りを含んだ一覧が60秒キャッシュされ、誰が開いても同じ誤りが出た。

## What Changes

- 受信箱のサービスは、一覧を求めた要求から切り離して、フォージに問い合わせる。1回の取得は30秒を上限とする
- 届かなかったフォージがある一覧は、返すがキャッシュしない。次の要求でフォージに問い合わせ直す

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- `inbox-api`: 一覧の取得を呼び出し側の取り消しから切り離す要件と、誤りを含む一覧をキャッシュしない要件を足す

## Impact

- `internal/inboxserve/serve.go`
- `internal/inboxserve/cache_test.go`
