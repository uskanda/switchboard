# Design

## Context

`static` は、資産を `http.ServeFileFS` で、`index.html` を `w.Write` で返していた。資産は `embed.FS` に埋め込まれており、`embed.FS` には更新時刻が無い。そのため `Last-Modified` は付かず、`ETag` も付いていなかった。Viteは `assets/` の下に、ハッシュ付きの名前で出力する。

## Goals / Non-Goals

**Goals:**

- 2回目以降の表示で、`assets/` の下の資産を取り直さない
- 初回の表示の転送量を、gzipで減らす
- `index.html` は毎回問い合わせさせ、変わっていなければ304で済ませる

**Non-Goals:**

- brotliへの対応
- `/api/*` とSSEの見出しの変更

## Decisions

### 圧縮はGo側で、初回の要求のときに行う

1ファイルぶんの返す内容（元のバイト列、gzipのバイト列、`Content-Type`、`ETag`）を `staticFile` にまとめる。`Server` はファイル名をキーにして、これをmutexとmapで持つ。最初の要求で読んで作り、次からは使い回す。`Assets` が差し替わる使い方は無いので、無効化はしない。

gzip版を作るのは、テキスト系の型で、圧縮して小さくなるときだけとする。テキスト系の型は `text/*`、`application/javascript`、`application/json`、`application/manifest+json`、`image/svg+xml` である。画像やフォントなど、すでに圧縮された形式はそのまま返す。

選ばなかった案は次のとおり。

- ビルド時に `.gz` を作って埋め込む案。バイナリに同じ内容が2通り入り、ビルドの手順も増える
- 要求のたびに圧縮する案。毎回CPUを使う

### ETagは中身のハッシュから作る

`embed.FS` に更新時刻が無いので、`Last-Modified` の代わりに、元のバイト列のSHA-256の先頭から `ETag` を作る。gzip版は `"<hash>-gz"` として、元の版と区別する。返すときは `http.ServeContent` に `bytes.Reader` を渡し、`ETag` をヘッダに置く。これで、`If-None-Match` による304とHEADの扱いはGoが受け持つ。

### RangeとVary

gzip版を返すときは、要求から `Range` の見出しを外してから `http.ServeContent` に渡す。圧縮後のバイト列に範囲を当てると、ブラウザが組み立てられないためである。gzip版のあるファイルには、どちらの版を返すときも `Vary: Accept-Encoding` を付ける。`Accept-Encoding` の `gzip` が `q=0` のときは、gzipを受け取れないものとして扱う。

### Cache-Control

パスが `assets/` で始まるファイルは `public, max-age=31536000, immutable` とする。`index.html`（どのパスへのフォールバックでも）と、ルートにあるそのほかのファイルは `no-cache` とする。ビルドされていないとき（`index.html` が無いとき）の503の応答は変えない。

## Risks / Trade-offs

- 資産をメモリに2通り持つ → 資産は数百KBで、gzip版はその3分の1ほどなので、増える量は小さい
- `assets/` の下にハッシュの付かないファイルが置かれると、1年間キャッシュされる → Viteの出力だけが `assets/` に入る作りなので、起きない
