# Tasks

## 1. 設定（`config`）

- [x] 1.1 `<inbox.refresh_interval>` を足す。無ければ30秒になること、10秒未満を拒むことを、失敗するテストから確かめる。`config/switchboard.example.yaml` に書く

## 2. 受信箱の型（`inbox`、`inboxapi`）

- [x] 2.1 `updated_at` と `notes` を、一覧の要素に足す。偽のフォージで、GitHubとGitLabの値が入ることをテストする
- [x] 2.2 1件の詳細に、issueの本文とコメントの一覧を足す。エージェントの投稿の `agent` と `kind`、マーカーを除くこと、人のコメントをそのまま返すことをテストする

## 3. 変化の知らせ（`inboxserve`、`inboxapi`）

- [x] 3.1 `/v1/events` と問い合わせのループを作る。次をテストする。つないだ直後に一覧が届くこと。一覧が変わったときだけ届くこと。購読者が0の間は問い合わせないこと。返答のあとで間隔を待たずに取り直すこと。ループの一覧がGETのキャッシュに入ること
- [x] 3.2 `inboxapi.Client` の `<Events>` を作る。テストのサーバーで、知らせを順に受け取ることと、切れたら誤りを返すことを確かめる
- [x] 3.3 `switchboard inbox serve` で、設定の間隔を渡す

## 4. WUIのサーバー（`web/internal/server`）

- [x] 4.1 `/api/events` を作る。受信箱のサービスの知らせを中継すること、届かないときに `event: unavailable` を送ること、`WriteTimeout` で切られないことをテストする

## 5. 画面（`web/ui`）

- [x] 5.1 `InboxLayout` で、ヘッダと2つのペインの組み方を作る。幅による出し分け、ヘッダの件数と接続の状態、`/` の右のペインの案内を、VitestとTesting Libraryでテストする
- [x] 5.2 `EventSource` の知らせによる一覧の書き換えと、選んだissueの取り直しを作る。書きかけが残ること、待ちでなくなったら送れなくなること、一覧から消えたことの表示をテストする
- [x] 5.3 `react-markdown` で、スレッドの表示を作る。Markdownの描画、生のHTMLと画像の扱い、リンクの属性、エージェントの印、着手と進捗の畳み方をテストする
- [x] 5.4 `DESIGN.md` の「Layout」を2つのペインに合わせて書き直す。幅360px、1024px、1280pxで画面を撮って確かめる

## 6. 文書と確認

- [x] 6.1 README.mdに、2つのペインの画面と自動更新、`<inbox.refresh_interval>` を書く
- [ ] 6.2 webhookのイベントを中継から受けて即時に取り直すことを、issueとして立てる
