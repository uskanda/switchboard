# Tasks

## 1. 素材

- [x] 1.1 `web/ui/public/`に`favicon.svg`、`favicon-waiting.svg`、`manifest.json`を置く
- [x] 1.2 `favicon.svg`から`apple-touch-icon.png`（180px）、`icon-192.png`、`icon-512.png`を書き出す。使ったコマンドをここに残す

  `favicon.svg`から角丸を外した（`rect`から`rx`を除いた）SVGを用意する。それを、地を墨色で塗って全面に広げるHTMLへ入れ、PlaywrightのChromiumのheadless shellで撮った。`<size>`を180、192、512にして3回実行する。

  ```sh
  chrome-headless-shell --no-sandbox --hide-scrollbars --force-device-scale-factor=1 \
    --window-size=<size>,<size> --screenshot=web/ui/public/<name>.png file://<page>.html
  ```

  HTMLの中身は次のとおり。

  ```html
  <style>html,body{margin:0;background:#1C1D1F;overflow:hidden}svg{display:block;width:100vw;height:100vh}</style>
  <!-- ここに、rxを除いたfavicon.svgの中身 -->
  ```
- [x] 1.3 `index.html`にicon、apple-touch-icon、manifest、apple-mobile-web-app-title、明暗2つのtheme-colorを書く
- [x] 1.4 `web/internal/server/server_test.go`で、`manifest.json`が`application/json`で返ることを確かめる。`favicon.svg`は`image/svg+xml`で返り、どちらも`index.html`に置き換わらないことを確かめる

## 2. ファビコンの切り替え

- [x] 2.1 `pages/usability.test.tsx`で、待ちの有無でファビコンのhrefが変わることを確かめる試験を先に書く
- [x] 2.2 `lib/title.ts`の`usePageTitle`で、`link[rel="icon"]`のhrefを差し替える

## 3. 一覧の時点

- [x] 3.1 `lib/freshness.test.ts`に試験を先に書き、`lib/freshness.ts`を作る。遅いほうの時刻を選ぶことと、今日と前日の形を確かめる。`liveUntil`がnullのときも確かめる
- [x] 3.2 `pages/live.test.tsx`に試験を先に書く。liveの間は出ず、切れると出て、liveに戻ると消えることを確かめる。EventSourceが無いときは`fetched_at`で出ることも確かめる
- [x] 3.3 `lib/inbox.tsx`に`liveUntil`を足し、`Header.tsx`に時点の表示を足す

## 4. 全体の確認

- [x] 4.1 `make verify`が通ることを確かめる
- [x] 4.2 `openspec validate add-web-favicon-and-freshness --strict`が通ることを確かめる
