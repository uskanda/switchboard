# screenshot Specification

## Purpose
計画の実行が、アプリの画面を撮り、変わる場所を注記した画像を作れるようにする。撮影は、switchboardの決まったサブコマンドで行い、ホストのChrome/Chromiumを使う。

## Requirements

### Requirement: 撮影のサブコマンド
`<switchboard shot>` は、URLと出力先のファイルを受け取り、そのページを撮ったPNGを出力先に書かなければならない（MUST）。
`<switchboard shot>` は、画面の幅と高さ、撮る前に待つ要素、ページ全体を撮るかどうかを引数で受け取れなければならない（MUST）。

#### Scenario: ページを撮る
- **WHEN** `http://127.0.0.1:5173/` で動くアプリに対して、幅390で `<switchboard shot>` を実行する
- **THEN** 出力先に、幅390の画面のPNGが書かれ、コマンドは0で終わる

### Requirement: 開けるURL
`<switchboard shot>` は、`http` か `https` で、ホストが `localhost`、`127.0.0.1`、`::1` のURLだけを開かなければならない（MUST）。
ほかのURLを受け取ったとき、`<switchboard shot>` は、ブラウザを起動せずに0以外の終了コードで終わらなければならない（MUST）。
ページの読み込む先のホストが、`localhost`、`127.0.0.1`、`::1` のどれでもないとき、`<switchboard shot>` は、その読み込みを止めなければならない（MUST）。

#### Scenario: 外のURL
- **WHEN** `https://example.com/` を渡して `<switchboard shot>` を実行する
- **THEN** ブラウザは起動せず、コマンドは0以外の終了コードで終わる

#### Scenario: 外へ読み込むページ
- **WHEN** 外のホストの画像を読み込むローカルのページを撮る
- **THEN** 外のホストへの要求は止められ、撮影は続く

### Requirement: 撮る前のJS
`<switchboard shot>` は、撮る前にページの中で実行するJSを、ファイルか引数で受け取れなければならない（MUST）。JSは、ページが読み込まれ、待つ要素が現れたあとに実行する。
JSの実行が例外で終わったとき、`<switchboard shot>` は、画像を書かずに0以外の終了コードで終わり、例外の内容を出さなければならない（MUST）。

#### Scenario: 文言を書き換えて撮る
- **WHEN** ボタンの文言を書き換えるJSを渡して撮る
- **THEN** 書き換えた文言のボタンの写る画像が書かれ、ソースのファイルは変わらない

### Requirement: 注記
`<switchboard shot>` は、撮った画像に、枠、矢印、文字の注記を描けなければならない（MUST）。位置は、CSSのセレクタか、画像の上の座標で指定する。
セレクタに合う要素の無いとき、`<switchboard shot>` は、画像を書かずに0以外の終了コードで終わらなければならない（MUST）。

#### Scenario: 要素を枠で囲む
- **WHEN** `#save` のセレクタに枠と「ここに移す」の文字の注記を付けて撮る
- **THEN** `#save` の要素の周りに枠と文字を描いた画像が書かれる

### Requirement: ブラウザの見つけ方
`<agent.browser>` が設定されているとき、`<switchboard shot>` は、そのパスのブラウザを使わなければならない（MUST）。
`<agent.browser>` が空のとき、`<switchboard shot>` は、PATHからブラウザを探さなければならない（MUST）。探す名前は、`chrome-headless-shell`、`chromium`、`chromium-browser`、`google-chrome` の順とする。完全なChromeは計画の実行のsandboxの中で起動できないので、ヘッドレスシェルを先に探す。
ブラウザが見つからないとき、`<switchboard shot>` は、0以外の終了コードで終わり、ブラウザが無いことを出さなければならない（MUST）。

#### Scenario: 設定で指定したブラウザ
- **WHEN** `<agent.browser>` に `~/.cache/ms-playwright/chromium-1243/chrome-linux/chrome` を設定して撮る
- **THEN** そのブラウザで撮られる
