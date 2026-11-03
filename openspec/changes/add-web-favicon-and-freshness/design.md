# Design

## Context

WUIの全応答にはCSP`default-src 'self'`が付く。Goのサーバーは`dist`の中のファイルをそのまま返し、無いパスには`index.html`を返す。受信箱のサービスは中身が変わったときだけ`/api/events`へ一覧を送るので、つながっている間に画面が持つ`fetched_at`は「最後に変わった時刻」である。動機はproposal.mdのWhyを参照。

## Goals / Non-Goals

**Goals:**
- 背面のタブでも、ファビコンで待ちに気付ける
- iOSとAndroidでホーム画面に追加できる
- 自動更新が切れている間、一覧がいつの時点のものかが分かる

**Non-Goals:**
- CSP、Goの静的配信、受信箱のサービスを変えること
- ビルドの依存を増やすこと

## Decisions

### 素材は`web/ui/public/`に置く

Viteは`public/`の中身を`dist`の直下へ写す。Goのサーバーはそのまま配るので、サーバーのコードは変えない。manifestの拡張子は`.json`とし、Goの既定のMIME表で`application/json`として返す。`.webmanifest`は表に無く、`nosniff`の下で`text/plain`になる恐れがある。

SVGの色はfill属性で直接書く。SVGの応答にも同じCSPが付くため、`<style>`による暗いテーマの切り替えには頼らない。待ちの点には墨色の縁を付け、明るいタブでも暗いタブでも読めるようにする。PNGは`favicon.svg`から一度だけ書き出す。iOSが角を丸めるので、PNGは角丸にせず全面を塗る。

### ファビコンの切り替えは`usePageTitle`で行う

`usePageTitle`は、すべての画面がすでに呼んでおり、待ちの件数を数えている。同じ件数で`link[rel="icon"]`のhrefを差し替えれば、タブのタイトルと基準がそろう。要素が無いとき（jsdomなど）は作る。

### 最後にliveだった時刻は`InboxProvider`が覚える

`InboxState`に`liveUntil`を足す。liveから離れる時点（`onerror`、`unavailable`、タブが裏に回って閉じるとき）で、直前がliveなら今の時刻を入れる。liveに戻っても値は残すが、表示には使わない。サーバーは変えない。

表示する文は純粋な関数（`lib/freshness.ts`）で作る。2つの時刻の遅いほうを取り、今日なら`HH:MM`、そうでなければ`M/D HH:MM`にする。ヘッダは`<time dateTime>`に元の値、titleに秒までの日時を入れる。表示は`aria-live="polite"`の状態の文の中に置く。切れたときは、状態と一緒に読み上げられる。

## Risks / Trade-offs

- PNGの書き出しに使った道具が手元に無いと、作り直せない。対策：書き出しのコマンドをtasks.mdに残す
- Android Chromeはhttpではアプリとしてインストールさせず、ショートカットとして追加する。対策：LANのhttpで配る前提なので受け入れる
