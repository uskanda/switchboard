# grilling 記録: add-web-favicon-and-freshness

issue `gh:uskanda/switchboard` #92の変更である。WUIにファビコン、ホーム画面に追加するためのmanifestとアイコン、一覧の時点の表示を足す。計画の段階で、エージェントがissueのコメント（6583855412）でQ1〜Q3を質問し、issueの持ち主がすべて推奨どおりに答えた（コメント6584232431）。そのやり取りをここに写す。

## 決定

| 決定 | 選択 | 出典 |
|---|---|---|
| ファビコンの作り方と、待ちがあるときの印 | A：静的なSVGを2つ（`favicon.svg`と`favicon-waiting.svg`）を`public/`に置き、待ちの件数（タブのタイトルと同じ`isWaiting`の数）が1以上のときに`<link rel="icon">`のhrefを差し替える。印は墨色（#1C1D1F）の角丸の正方形に白い記号を置き、待ちがあるときは右上にアンバー（#F5B35C）の点を足す。色はfill属性で直接書く。CSPの`default-src 'self'`を緩めずに済む。失敗の理由で点を赤にする案（B）は、16pxで見分けにくい。canvasで数字を描く案（C）はCSPを緩めることになる | issue #92 コメント6583855412 Q1（A） |
| ホーム画面に追加するためのmanifestとアイコン | A：`manifest.json`（name「SwitchBoard Inbox」、short_name「SwitchBoard」、start_url「/」、display「standalone」、theme_colorとbackground_colorはsurfaceの値）と、SVGから一度だけ書き出したPNG（180pxのapple-touch-icon、manifestの192pxと512px）をコミットする。ビルドの依存は増やさない。拡張子は`.webmanifest`にせず`.json`とし、Goの既定のMIME表で返るようにする | issue #92 コメント6583855412 Q2（A） |
| 最終更新の時刻を、いつ、どの時刻で出すか | C：自動更新がliveでない間だけ出す。時刻は`fetched_at`と、接続が最後にliveだった時刻の遅いほうを使い、「HH:MM時点の一覧」と出す。今日でなければ「M/D HH:MM」とする。liveの間は一覧が最新なので出さない。受信箱のサービスは中身が変わったときだけ一覧を送るので、`fetched_at`だけでは実際より古く見える | issue #92 コメント6583855412 Q3（C） |
| 計画の承認 | 計画を変えずに承認 | issue #92 コメント6584276601（`/ok`） |

## 後回しにしたもの

- 待ちに失敗の理由が含まれるときに印の色を変えること（Q1でBを選ばなかった）
- PNGをビルドのたびに生成すること（Q2でBを選ばなかった）

## 状態

frontierは空。共有理解は2026-11-02に確認済み。
