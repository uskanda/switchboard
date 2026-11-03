# Proposal

## Why

受信箱のWeb表示（WUI）は、スマホではブラウザのタブとして開くしかなく、通知もTelegramを経由する。issue #99のとおり、ExpoでAndroidのアプリを作る。アプリはホーム画面から全画面で開け、ネイティブのプッシュ通知から計画の承認とマージへ進める。画面はWUIそのものを使い、WUIの変更へ追随し続けられるようにする。

## What Changes

- `app/` に、Expo SDK 58のAndroidのアプリを足す。アプリはWUIをWebViewで開く殻とし、画面を書き直さない。ネイティブで書くのは、WUIのURLの設定、FCMの受信、通知を押したときの画面の移動、戻るボタンの扱いである
- 受信箱のサービス（`switchboard inbox serve`）が、一覧の変化を見て、FCM HTTP v1でプッシュ通知を送る。載せるのは、issueの参照、待ちの理由、issueのタイトルだけで、質問の本文は載せない。エージェントが送るTelegramの通知は変えず、両方を併用する
- 計画の承認待ちの通知に「承認」のボタンを、完了の通知に「マージ」のボタンを付ける。承認は、通知に入れた待ちの投稿の時刻が最新と一致するときだけ受け付ける。マージは、アプリを開いてWUIの確認のダイアログを経る
- 受信箱のAPIに、アプリの登録（`<POST /v1/devices>`）と、計画の承認（`<POST /v1/approve>`）を足す。`switchboard-web` は、どちらも返答と同じ `Origin` と `Host` の検査をかけて取り次ぐ。アプリは、設定したWUIのURLを `Origin` に明示して送る
- FCMの設定（`<inbox.notify.fcm>`）があるとき、受信箱のサービスは、画面がつながっていなくても `<inbox.refresh_interval>` ごとに一覧を取り直す
- WUIは、1件の画面のURLの問い合わせ（`<?action=merge>`）で、マージの確認のダイアログを開く
- `make verify` に `app/` の型の検査、lint、テストを足す。APKのビルドは `make app-apk` に分ける
- ADRを足す（アプリの形、FCMでフォージの外に出す内容、計画の「承認」を原則3の例外とすること）。PRODUCT.mdのPlatformと原則3を改める

## Capabilities

### New Capabilities

- `android-app`：WUIをWebViewで開くAndroidのアプリ。WUIのURLの設定、FCMの登録と受信、通知のボタンの処理、ビルドと署名
- `push-notify`：受信箱のサービスが一覧の変化を見てFCMで送るプッシュ通知。送る条件、載せる内容、送った記録、失敗の扱い

### Modified Capabilities

- `inbox-api`：アプリの登録と計画の承認の口を足す。FCMの設定があるときは、画面がつながっていなくても一覧を取り直す
- `web`：アプリの登録と計画の承認のPOSTを、返答と同じ検査で取り次ぐ。URLの問い合わせでマージの確認のダイアログを開く
- `config`：`<inbox.notify.fcm>` を足す。`<inbox.refresh_interval>` の意味を、FCMの設定があるときに広げる

## Impact

- 新しいディレクトリ：`app/`（Expo、React Native、react-native-webview、expo-notifications、expo-build-properties）
- Go：`internal/inboxserve`、`inboxapi`、`internal/config`、FCMの送信の新しいパッケージ、`web/internal/server`
- WUI：`web/ui` の1件の画面（マージのダイアログを開く問い合わせ）
- 外部：Firebaseのプロジェクト、サービスアカウントの鍵（Secret Service）、`google-services.json`（リポジトリに入れない）
- ビルド：`Makefile` の `verify` と新しいターゲット。JDK 17とAndroid SDKは `make app-apk` だけに要る
- 文書：`docs/DECISIONS.md`、`docs/ARCHITECTURE.md`、`docs/SECURITY.md`、`docs/ROADMAP.md`（検証項目）、`PRODUCT.md`、README
