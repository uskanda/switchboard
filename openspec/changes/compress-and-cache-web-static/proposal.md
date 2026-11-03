# Proposal

## Why

Web UIの静的ファイルは、圧縮もキャッシュの見出しも無く返していた（#91）。監査の時点では、ページを開くたびにJSを541KB取り直していた（gzipなら168KB）。`Cache-Control`、`ETag`、`Last-Modified` が無く、2回目以降も全量を転送する。Telegramの通知から開くたびに、この転送が起きていた。

## What Changes

- テキスト系の静的ファイルを、ブラウザがgzipを受け取れるときにgzipで返す。圧縮はファイルごとに初回の要求で1回だけ行い、結果をメモリに持つ
- `assets/` の下のハッシュ付きの資産には `Cache-Control: public, max-age=31536000, immutable` を付ける
- `index.html` とルートにあるそのほかのファイルには `Cache-Control: no-cache` を付ける
- すべての静的ファイルに、中身のハッシュから作った `ETag` を付け、`If-None-Match` が一致すれば304を返す
- ビルドの手順と、`/api/*` とSSEの見出しは変えない

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- `web`: 静的ファイルの圧縮とキャッシュ

## Impact

- `web/internal/server/server.go`（`static`）
- `web/internal/server/server_test.go`
