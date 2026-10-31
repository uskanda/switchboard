# Tasks

## 1. 設定とフォージ（`config`、`forge`）

- [x] 1.1 `<notify>` 節（`<notify.telegram>` の `chat_id` と `token_ref`、`<notify.web_url>`）を読む。片方だけの `<notify.telegram>`、絶対URLでない `<notify.web_url>` を拒むこと、`<notify>` の無い設定を今までどおり読めることを、失敗するテストから確かめる
- [x] 1.2 `forge.IssueSummary` に `URL` を足し、GitHubは `html_url`、GitLabは `web_url` から埋める。偽物のフォージも決まった形のURLを返す。記録したHTTPのやり取りでテストする

## 2. 通知（`notify`、`agent`）

- [x] 2.1 `internal/notify` で、Telegramの `sendMessage` への送信を作る。送る項目（`chat_id`、`text`、`parse_mode` が無いこと）、10秒の打ち切り、誤りの文にトークンが現れないことを `httptest` で確かめる
- [x] 2.2 待ちと完了の通知の本文を作る。待ちには、プロジェクト、issue番号、タイトル、理由、300文字に切った質問の抜粋、issueのURLを載せる。完了には、PRかMRのURLとissueのURLを載せる。`<notify.web_url>` があればWUIへのリンクを足す。本文をテストで確かめる
- [x] 2.3 エージェントの `writeBack` から通知を呼ぶ。偽のフォージと偽の通知で、次をテストする。投稿したときだけ1通送ること、二重投稿の防止で投稿しなかったときは送らないこと、失敗しても待ちのコメントとラベルが付き、監査ログに `<notify.failed>` が残ること

## 3. 受信箱のAPI（`inbox`、`inbox-api`）

- [x] 3.1 `internal/inbox` を分け直す。フォージごとの誤りを返し、行番号を記録しない `Collect` を足す。`Show` を `Detail` と `PrintDetail` に分ける。`Reply` の待ちでない誤りを `ErrNotWaiting` にする。今のCLIのテストが変わらず通ることを確かめる
- [x] 3.2 公開のパッケージ `inboxapi/` に、要求と応答の型、誤りの型、Unixソケットにつなぐ `Client` を作る
- [x] 3.3 `internal/inboxserve` で、`/v1/inbox`、`/v1/issue`、`/v1/reply` を作る。届かなかったフォージの扱い、60秒のキャッシュと返答のあとの破棄、待ちでないときの409を、偽のフォージと `<inboxapi>` の `Client` でテストする
- [x] 3.4 `switchboard inbox serve` を組み立てる。ソケットの場所の決め方（`--socket`、`RUNTIME_DIRECTORY`、`XDG_RUNTIME_DIR`）、0600の権限、古いファイルの置き換え、受信箱のトークンだけを読むことをテストで確かめる

## 4. WUI（`web`）

- [ ] 4.1 `ui-guidelines` に従い、リポジトリ直下に `<DESIGN.md>`（見た目の基準）と `<PRODUCT.md>`（誰が何のために使うか）を書く。デスクトップとスマホの両方を対象にする
- [x] 4.2 `web/` のGoのモジュールを作る。`<web.yaml>` の読み込みで、`listen` が無いとき、ホストが空、`0.0.0.0`、`::`、IPアドレスでないときに起動を拒むことを、失敗するテストから確かめる
- [x] 4.3 WUIのバックエンドを作る。`/api/inbox`、`/api/issue`、`/api/reply` を `<inboxapi>` に中継し、返答のPOSTで `Content-Type`、`Origin`、`Host` を検査する。外のオリジン、違う `Host`、`Origin` の無い要求が403になり投稿されないこと、受信箱のサービスに届かないときの応答を、偽の `inbox serve` でテストする。応答のセキュリティのヘッダーも確かめる
- [ ] 4.4 `web/ui` にVite、React、TypeScript、Tailwind、shadcn/uiの土台を作る。`pnpm typecheck`、`pnpm lint`、`pnpm test`、`pnpm build` を用意し、ビルドの成果物を `go:embed` で埋め込む。`dist/` が空でもGoのテストが通ることを確かめる
- [ ] 4.5 一覧と1件の画面を作る。項目の表示、届かなかったフォージの警告、返答の送信、待ちでないときの表示、HTMLを含むタイトルが文字のまま出ることを、VitestとTesting Libraryで確かめる。幅360pxとデスクトップの幅で、横へのスクロール無しに使えることを、ブラウザで確かめる

## 5. サービス（`service`）

- [ ] 5.1 `service install --inbox`、`uninstall --inbox`、`status --inbox` を作る。各フォージの `inbox_token_ref` だけがクレデンシャルになること、unitに `RuntimeDirectory=switchboard` が入ることをテストで確かめる
- [ ] 5.2 `service install --web`、`uninstall --web`、`status --web` を作る。次をテストで確かめる。`PATH` から `<switchboard-web>` を探して絶対パスで書くこと、見つからなければsudoを呼ばないこと、クレデンシャルと `RuntimeDirectory=` が無いこと
- [ ] 5.3 エージェントのサービスで、`<notify.telegram>` の `token_ref` もクレデンシャルにする。受信箱のトークンが入らないままであることをテストで確かめる

## 6. 検証の入口と文書

- [ ] 6.1 `make verify` に `verify-web` を、`make build` に `bin/switchboard-web` を足す。CIに、Node.jsとpnpmの準備を足す
- [ ] 6.2 DECISIONS.mdに、ADR-036（WUIに限ってLANで受信ポートを開く、当面は認証しない）とADR-037（ADR-010の範囲を核に限り、WUIを別のモジュールにする）を足す。AGENTS.mdとSECURITY.mdのT5を、この例外に合わせて書き直す
- [ ] 6.3 README.mdに、Telegramのトークンの置き方と `<notify>` の設定を書く。`inbox serve` とWUIの設置、`<web.yaml>` の書き方、`listen` にIPアドレスを書く理由も書く
- [ ] 6.4 WUIの認証を課題とするGitHubのissueを作る。LANの中の別の機器から `uskanda` として返答でき、それがエージェントへの信頼された指示になることを書く

## 7. 結合の確認

- [ ] 7.1 usknasで、利用者にTelegramのbotのトークンを `switchboard/telegram` に置いてもらう。設定に `<notify>` を足し、`service install`、`service install --inbox`、`service install --web` を実行してもらう。防火壁がWUIのポートを塞いでいないかも見る
- [ ] 7.2 GitLabのsandboxでissueを待ちにして、Telegramに通知が届くことを確かめる。通知のリンクから、スマホのブラウザで受信箱を開く。返答すると実行が再開し、完了の通知が届くことも確かめる。デスクトップのブラウザでも同じ画面を開いて確かめる。結果はROADMAP.mdに記録する
