# Tasks

## 1. 文書

- [x] 1.1 `docs/DECISIONS.md` にADR-047を足す。WebViewの殻、受信箱のサービスからのFCM、Googleに渡す内容、原則3の例外を書く（design D1、D4、D5、D6）
- [x] 1.2 PRODUCT.mdのPlatformにAndroidのアプリを足し、原則3に計画の「承認」のボタンの例外を書く
- [x] 1.3 `docs/ARCHITECTURE.md` の全体像と2.4節に、アプリとFCMの経路を書く。`docs/SECURITY.md` のT5にアプリのPOST、T9に通知の内容、4節にFCMの鍵を書く
- [x] 1.4 `docs/ROADMAP.md` の検証項目に、2つの項目を「未確認」として足す。1つは、dataだけのFCMのメッセージをカテゴリ付きで表示できるか（design D5）である。もう1つは、終了したアプリで「承認」のボタンの処理が動くか（design D6）である
- [x] 1.5 `docs/DESIGN.md` の受信箱のサービスとWUIの節に、通知の差分、宛先の登録、承認の口を書く

## 2. 設定（`internal/config`）

- [x] 2.1 `<inbox.notify.fcm>`（`project_id`、`credentials_ref`）を足し、片方だけの設定を拒むことと、無い設定が読めることをテストで確かめる
- [x] 2.2 `config/switchboard.example.yaml` に `<inbox.notify.fcm>` の例と説明を足し、例の設定を読むテストが通ることを確かめる

## 3. FCMの送信（`internal/push`）

- [x] 3.1 FCMのクライアントを作る。サービスアカウントの鍵からRS256のJWTを作り、アクセストークンに換えて期限の5分前まで使い回す。httptestのトークンの口で、署名の検証、使い回し、期限での取り直しをテストで確かめる（design D4）
- [x] 3.2 FCMのdataだけの優先度の高いメッセージを送る処理を作る。待ち、計画の承認待ち、完了の3つについて、`data` の項目とカテゴリをテストで確かめる。質問の本文を含まないことも確かめる（design D5）
- [x] 3.3 `push-devices.json` に宛先の登録を作る。登録し直し、上限10での古いものの削除、空と長すぎるトークンの拒否をテストで確かめる
- [x] 3.4 `push-sent.json` に送った記録を置き、差分の処理を作る。新しい待ち、状態の変化、理由と開始時刻の変化、変わらないもの、実行中と順番待ち、起動して最初の一覧、一覧から消えたissueをテストで確かめる
- [x] 3.5 FCMの送信の失敗の扱いを作る。監査ログの `push.failed`、`UNREGISTERED` のトークンの削除、10秒の打ち切り、鍵とトークンがログに出ないことをテストで確かめる

## 4. 受信箱のサービス（`inboxapi`、`internal/inboxserve`、`cmd/switchboard`）

- [x] 4.1 `inboxapi` に、登録と承認の要求の型と、クライアントの `RegisterDevice` と `Approve` を足し、クライアントのテストで確かめる
- [x] 4.2 `internal/inboxserve` に `POST /v1/devices` を足し、登録と拒否をテストで確かめる
- [x] 4.3 `internal/inboxserve` に `POST /v1/approve` を足す。承認の判定は、`internal/inbox` の詳細と返答を使って `internal/inboxserve` で行う。理由が `plan` で、開始時刻も一致するときに限り、承認の語を投稿する。開始時刻の違い、理由の違い、待ちでないときに `stale` を返すことをテストで確かめる（design D6）
- [x] 4.4 `<inbox.notify.fcm>` があるときは、画面が無くても `loop` を動かし続ける。中継のイベントでも取り直し、一覧を差分の処理へ渡す。設定が無いときの今の動きが変わらないことも含めて、テストで確かめる
- [x] 4.5 `switchboard inbox serve` の組み立てで、鍵を `secret.Resolver` で読み、`internal/push` を渡す。鍵が読めないときに起動が失敗し、誤りに鍵の内容が出ないことを `cmd/switchboard` のテストで確かめる

## 5. WUI（`web/internal/server`、`web/ui`）

- [x] 5.1 `web/internal/server` に `<POST /api/devices>` と `<POST /api/approve>` を足す。`sameOrigin` の検査、取り次ぎ、誤りの対応をテストで確かめる
- [x] 5.2 1件の画面で、`<action=merge>` の問い合わせがあれば詳細を読んだあとでマージの確認のダイアログを開き、閉じたら問い合わせを外す。マージのボタンが出ないissueでは誤りのトーストを出す。vitestのテストで確かめる（design D7）
- [x] 5.3 DESIGN.mdのマージの節に、問い合わせで開くダイアログを書き足し、`designmd lint DESIGN.md` が誤り0で通ることを確かめる

## 6. アプリ（`app/`）

- [x] 6.1 `app/` にExpo SDK 58、TypeScriptのプロジェクトを作る。依存には `react-native-webview`、`expo-notifications`、`expo-build-properties`、AsyncStorageを足す。`android/` を `.gitignore` に入れる。`pnpm install --frozen-lockfile` と型の検査が通ることを確かめる（design D2）
- [x] 6.2 `Makefile` に `verify-app`（型の検査、ESLint、jest-expoのテスト）を足して `verify` から呼ぶ。`make verify` が通ることを確かめる
- [x] 6.3 URLの検証と保存、ネイティブのPOSTの `Origin` の付け方を、ReactのコンポーネントとNativeのAPIから分けた関数として作り、jestのテストで確かめる（design D3）
- [x] 6.4 `app/` の設定の画面とWUIの画面を作る。URLが無いときの初回の画面、届かないときの表示、オリジンの外のリンクを既定のブラウザで開くこと、戻る操作をjestのテストで確かめる（design D1）
- [x] 6.5 `expo-notifications` で、通知の許可、トークンの登録と更新、カテゴリ `<plan>` と `<done-merge>` の登録を作る。登録のPOSTの中身をjestのテストで確かめる
- [x] 6.6 `expo-notifications` で通知の応答を作る。本体は `/issue/<ref>` を開き、「マージ」は `<action=merge>` 付きで開き、「承認」は `<POST /api/approve>` を送る。断られたらローカルの通知を出す。終了していたアプリの起動では最後の応答を読む。それぞれをjestのテストで確かめる（design D6、D7、D8）
- [x] 6.7 `app/app.config.ts` とconfig pluginで、平文のHTTP、署名、`google-services.json` を環境変数から設定する。`make app-apk` を足し、変数が足りないときにGradleの前で止まることを確かめる
- [x] 6.8 READMEに、Firebaseの準備、鍵の作り方、`make app-apk` の使い方を書く。URLは `listen` と同じ `IP:ポート` で入力することも書く。書いた手順でAPKができることを確かめる

## 7. 実機での確認

- [ ] 7.1 Androidの実機にAPKを入れ、URLの設定、WUIの表示、戻る操作、フォージのリンクが既定のブラウザで開くことを確かめる
- [ ] 7.2 usknasの常駐の受信箱のサービスに `<inbox.notify.fcm>` を設定し、待ちと完了の通知がTelegramとアプリの両方に届くことを確かめる。通知に質問の本文が無いことも確かめる
- [ ] 7.3 Androidの実機で、計画の承認待ちの通知の「承認」で `/ok` が付くこと、古い通知では断られることを確かめる。アプリを終了させた状態でも試し、結果をROADMAPの検証項目に書く。動かなければdesign D6の代わりの形に直す
- [ ] 7.4 Androidの実機で、完了の通知の「マージ」でマージの確認のダイアログが開き、マージできることを確かめる。dataだけのメッセージの表示の結果をROADMAPの検証項目に書く
