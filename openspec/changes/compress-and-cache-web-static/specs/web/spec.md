## ADDED Requirements

### Requirement: 静的ファイルの圧縮とキャッシュ
テキスト系の型は `text/*`、`application/javascript`、`application/json`、`application/manifest+json`、`image/svg+xml` とする。
WUIのサーバーは、要求の `Accept-Encoding` がgzipを受け付けるとき、テキスト系の静的ファイルをgzipで返さなければならない（MUST）。そのとき `Content-Encoding: gzip` を付ける。
gzip版を持つファイルには、どちらの版を返すときも `Vary: Accept-Encoding` を付けなければならない（MUST）。すでに圧縮された形式（画像など）は、gzipにしてはならない（MUST NOT）。
パスが `assets/` で始まる静的ファイルには、`Cache-Control: public, max-age=31536000, immutable` を付ける（MUST）。
`index.html`（どのパスへのフォールバックでも）と、ルートにあるそのほかの静的ファイルには、`Cache-Control: no-cache` を付けなければならない（MUST）。
WUIのサーバーは、静的ファイルに、中身のハッシュから作った `ETag` を付けなければならない（MUST）。要求の `If-None-Match` が一致すれば、304を返さなければならない（MUST）。gzip版には、元の版とは別の `ETag` を付ける。

#### Scenario: gzipを受け取れるブラウザがJSを取る
- **WHEN** `Accept-Encoding: gzip` を付けて `assets/` の下のJSを要求する
- **THEN** `Content-Encoding: gzip` と `Vary: Accept-Encoding` が付き、本文を展開すると元のJSと同じになる

#### Scenario: gzipを受け取れない要求
- **WHEN** `Accept-Encoding` を付けずに `assets/` の下のJSを要求する
- **THEN** 元のままのバイト列が返り、`Content-Encoding` は付かない

#### Scenario: すでに圧縮された形式
- **WHEN** `Accept-Encoding: gzip` を付けてPNGの画像を要求する
- **THEN** 元のままのバイト列が返り、`Content-Encoding` は付かない

#### Scenario: ハッシュ付きの資産のキャッシュ
- **WHEN** `assets/` の下のファイルを要求する
- **THEN** `Cache-Control: public, max-age=31536000, immutable` が付く

#### Scenario: index.htmlのキャッシュ
- **WHEN** `/`、`/index.html`、またはフォールバックするパス（例：`/issue/x`）を要求する
- **THEN** `index.html` が `Cache-Control: no-cache` と `ETag` を付けて返る

#### Scenario: 変わっていないファイルの問い合わせ
- **WHEN** 前の応答の `ETag`（gzip版のものを含む）を `If-None-Match` に入れて同じファイルを要求する
- **THEN** 304が返り、本文は空になる

#### Scenario: HEADの要求
- **WHEN** 静的ファイルをHEADで要求する
- **THEN** GETと同じ見出しが返り、本文は空になる
