# Tasks

## 1. 前提の確認

- [x] 1.1 V16を確かめる。`gl.uskan.dev` の `nas/switchboard-sandbox` で、APIと画面のそれぞれで署名トークンを設定できるかを調べる。届くヘッダー（`webhook-id`、`webhook-timestamp`、`webhook-signature`、`Idempotency-Key`）と、署名の鍵の形と計算方法を記録する。結果をROADMAP.mdのV16に書き、design.mdの3節と6節を実物に合わせる

## 2. 設定（`config`）

- [x] 2.1 `<relay>` 節（受け口と配信口のアドレス、`<webhook_url>`、webhookの一覧、端末のトークンのハッシュの一覧）を読む。`hook-key` の重複、知らないプロジェクト、値と見える文字列を拒否することを、失敗するテストから確かめる
- [x] 2.2 `agent` の `<relay>`（URLと `token_ref`）と `<reconcile_interval>`（既定10分、`poll_interval` 以上）を読む。`<relay>` の無い設定を今までどおり読めることを、テストで確かめる

## 3. 中継の受信（`relay`）

- [x] 3.1 GitHubの署名の検証と抽出を作る。正しい署名、違う署名、対象外のイベント、本文が抽出の結果に残らないことを、記録したペイロードでテストする
- [x] 3.2 GitLabの署名トークンの検証と抽出を、1.1で確かめた形に合わせて作る。破棄するもの（古いタイムスタンプ、MRへのNote Hook、`<gitlab_token>` の無いプロジェクトに届いた `X-Gitlab-Token` だけのwebhook）をテストする
- [ ] 3.3 受け口のハンドラを組む。ボディの上限（413）、知らない `hook-key`（404）、署名の失敗（401）、配信IDの重複（200で配らない）を `httptest` で確かめる。ログに本文とユーザー名が出ないことも確かめる

## 4. 中継の配信（`relay`）

- [ ] 4.1 SSEの配信口を作る。bearerトークンのハッシュでの認証（無いと401）、全端末への同報、1,000件のリングバッファ、`Last-Event-ID` での再送、30秒ごとの `: ping` をテストする
- [ ] 4.2 `switchboard relay serve` を組み立てる。2つのリスナーを設定のアドレスで待ち、シークレットを `secret.Resolver` で読む。webhookを送ってSSEにイベントが1件届くことを、結合のテストで確かめる

## 5. エージェントの中継モード（`relayclient`、`agent`）

- [ ] 5.1 `internal/relayclient` で、SSEの購読、`Last-Event-ID`、1秒から60秒までの再接続の間隔を作る。切断と再接続を `httptest` のサーバーで確かめる
- [ ] 5.2 エージェントに中継モードを足す。担当でないプロジェクトのイベントを捨てること、同じプロジェクトのイベントを3秒まとめて1回だけ取り直すことを、偽のフォージと偽の中継でテストする
- [ ] 5.3 照合の間隔の切り替えを作る。つながっている間は `<reconcile_interval>`、切れたらすぐ `poll_interval` に戻ること、つながったら1回照合することをテストで確かめる。イベントによる取り直しと `Poll` が同時に走らないことも確かめる

## 6. webhookとトークンの管理（`forge`、`relay`）

- [ ] 6.1 `<forge.Hooks>`（`CreateHook`、`DeleteHook`）を、GitHub、GitLab、偽物に実装する。送る項目（URL、イベント、シークレット）を、記録したHTTPのやり取りで確かめる
- [ ] 6.2 `relay hook add` と `relay hook remove` を作る。権限のあるトークンでは登録してシークレットの値を出さないこと、403か404では画面での手順（GitLabでは許可リストのアドレスを含む）を出すこと、設定に足す項目を表示することをテストする
- [ ] 6.3 `relay token add` を作る。トークンの値がSecret Serviceにだけ置かれ、表示されるのがハッシュだけであることをテストで確かめる

## 7. サービス（`service`）

- [ ] 7.1 `service install --relay` と `service uninstall --relay` を作る。中継のunitに、webhookのシークレットだけがクレデンシャルとして入ることをテストで確かめる
- [ ] 7.2 エージェントのサービスで、`<agent.relay>` の `token_ref` もクレデンシャルにする。受信箱のトークンが入らないままであることをテストで確かめる

## 8. 文書

- [ ] 8.1 DECISIONS.mdで、ADR-007とADR-017を決め、ADR-035（中継の認証とシークレット）を足す。DESIGN.md 4.7節を、決めた形に合わせる
- [ ] 8.2 README.mdに、中継の設置、webhookの登録、GitLabの許可リストの設定、止まったwebhookの有効化を書く
- [ ] 8.3 GitHubの実物での確認を、GitHubのissueにする

## 9. 結合の確認

- [ ] 9.1 usknasで、利用者に設置してもらう。手順は、`relay token add` と `relay hook add gl:nas/switchboard-sandbox` の実行、GitLabの許可リストの設定である。続けて、`service install --relay` と `service install` を実行する。受信箱から返答して、実行の開始までが10秒以内であることを監査ログの時刻で確かめる。中継を止めたら30秒のポーリングで処理が続くことと、起動し直したら再接続することも確かめる。結果はROADMAP.mdに記録する
