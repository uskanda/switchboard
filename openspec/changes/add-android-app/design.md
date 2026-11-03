# Design

## Context

動機はproposal.mdの「Why」を参照。この設計を縛る今の作りは次のとおり。

- WUIは `web/ui` にあり、Vite、React 19、Tailwind v4、react-router v7で作る。`switchboard-web` がLANのIPアドレスだけで配る（ADR-036、ADR-037）。一覧の更新はSSE（`/api/events`）で受ける
- `switchboard-web` の `sameOrigin` は、POSTの本文がJSONであることを確かめる。`Origin` と `Host` が `listen` に一致することも確かめる。利用者の認証は無い（#15）
- 受信箱のサービスの `loop` は、SSEの口に画面がつないでいる間だけ、`<inbox.refresh_interval>` ごとに一覧を取り直す
- Telegramの通知は、各端末のエージェントが自分の担当のissueについて送る（`internal/notify`）
- Expo SDK 58（2026-10-15、React Native 0.88、React 19.3、targetSdk 36）が最新である。New Architectureは必須で、SDKは年3回出る
- React Nativeの `fetch` は、Androidで `Origin` を付けない。Androidは平文のHTTPを既定で止める
- この端末には、Android SDK 36、NDK 27、JDK 17（mise）が入っている

## Goals / Non-Goals

**Goals:**

- WUIの変更に、アプリのビルドし直しなしで追随する
- ネイティブで書くコードを、URLの設定、通知、戻る操作に限る
- 通知でフォージの外に出す情報を、参照、理由、タイトルに限る

**Non-Goals:**

- 画面のネイティブでの書き直し、iOS、家の外からの利用（grilling.mdの「後回しにしたもの」）
- `switchboard-web` の `Origin` の検査を変えること
- EASとExpoのアカウントを使うこと

## Decisions

### D1. WUIをWebViewで開く殻にする（Q2、Q7）

アプリは、保存したWUIのURLを `react-native-webview` で全画面に開く。画面のコードは `web/ui` の1つだけで、WUIを配り直せばアプリにも反映する。

ほかの案は、Expo Routerとreact-native-webで `web/ui` を書き直す形と、DOM components（`'use dom'`）で部品を取り込む形だった。前者は、Tailwind v4をネイティブで使うためのNativeWind v5がまだRCで、UIとテストを全部書き直すことになる。後者は、ViteのビルドをMetroへ移す必要があり、react-routerが中で動くかを確かめられていない。どちらも、追随の手間を今より増やすので採らなかった。

アプリの画面は2つだけにする。

- 設定の画面：WUIのURLの入力と保存。URLが無いときの初回の画面を兼ねる
- WUIの画面：WebViewで開く。届かないときは、届かないことと設定の画面へのボタンを出す

画面の切り替えは状態の1つの値で行い、Expo Routerは使わない。URLは `@react-native-async-storage/async-storage` に保存する。秘密ではないので、暗号化した保存先は使わない。

### D2. アプリの置き場所とビルド（Q3、Q6、Q16）

`app/` に、`web/ui` と別のpnpmのプロジェクトとして置く。SDK 54以降はpnpmの隔離したインストールを扱えるので、`nodeLinker` は既定のままにする。

`android/` はコミットしない。`make app-apk` が次の順で作る。

1. `app/` で `pnpm install --frozen-lockfile`
2. `expo prebuild --platform android --clean`
3. `android/` で `./gradlew assembleRelease`

署名と `google-services.json` は、`app/app.config.ts` とconfig pluginが環境変数から読む。変数は次の5つとする。

- `<SWITCHBOARD_APP_KEYSTORE>`
- `<SWITCHBOARD_APP_KEYSTORE_PASSWORD>`
- `<SWITCHBOARD_APP_KEY_ALIAS>`
- `<SWITCHBOARD_APP_KEY_PASSWORD>`
- `<SWITCHBOARD_APP_GOOGLE_SERVICES>`

足りなければ、`make app-apk` がGradleを動かす前に止まる。

平文のHTTPは、`expo-build-properties` の `usesCleartextTraffic` で許す。特定のアドレスだけを許すnetwork security configは、プラグインの文書に無いので使わない。アプリが開くのは利用者が入力したURLだけなので、許す範囲は広がっても、通信の先は増えない。

`make verify` には `verify-app` を足し、`app/` の型の検査、ESLint、jest-expoのテストを動かす。JDKとAndroid SDKの要る `make app-apk` は、`verify` に入れない。

### D3. ネイティブからのPOSTは `Origin` を明示する（Q11）

通知のボタンと登録は、WebViewを開かずにネイティブの `fetch` で送る。アプリは、本文をJSONにし、`Origin` に保存したURLのオリジンを付ける。`Host` は、OkHttpがURLから付けるので、保存したURLが `listen` と同じ `IP:ポート` なら一致する。設定の画面には、`web.yaml` の `listen` と同じ形で入力することを書く。

サーバーに別の検査（端末ごとの合言葉など）を足す案は採らなかった。`Origin` の検査は、ブラウザの上の別のページからのCSRFとDNS rebindingを防ぐためにある。ネイティブのアプリはその対象ではない。LANの中の機器なら送れることは、ADR-036で受け入れた範囲のままである。

### D4. 受信箱のサービスがFCMで送る（Q8、Q14、Q15）

新しいパッケージ `<internal/push>` を足し、次の3つを受け持たせる。

- FCM HTTP v1のクライアント：サービスアカウントの鍵でRS256のJWTを作り、`oauth2.googleapis.com` でアクセストークンに換える。トークンは期限の5分前まで使い回す。`fcm.googleapis.com/v1/projects/<project_id>/messages:send` に送る。Goの依存は増やさず、標準ライブラリの `crypto/rsa` で署名する
- 宛先の登録：`<push-devices.json>` を状態のディレクトリに置く。上限は10とし、超えたら登録の古いものを消す
- 送った記録と差分：`<push-sent.json>` に、ref、状態、理由、待ちの開始時刻を置く

`<inbox.notify.fcm>` があるとき、受信箱のサービスは、画面の有無にかかわらず `loop` を動かし続け、取り直した一覧を差分の処理に渡す。中継のイベントでも取り直す。起動して最初の一覧では、記録に無いものを記録だけする。

Expoのプッシュの中継（`exp.host`）を使う案は、Expoのアカウントと、そこへFCMの鍵を預けることが要るので採らなかった。各端末のエージェントが送る案は、全端末にFCMの鍵とアプリのトークンを置くことになるので採らなかった。

この形の代わりに、受信箱のサービスがフォージを読む回数が増える。間隔が30秒なら、GitHubのSearch APIは1分に2回で、上限の30回に収まる。

### D5. 通知はdataだけのFCMのメッセージにする（Q9、Q12）

FCMの `notification` のメッセージは、アプリが裏にいるとAndroidが直接出し、ボタンを付けられない。そのため、`data` だけの優先度の高いメッセージを送り、`expo-notifications` に表示させる。`data` には、表示の文（タイトルは `o/r#12 question`、本文はissueのタイトル）、カテゴリ、ref、状態、理由、待ちの開始時刻を入れる。完了には、PR/MRの番号とコミットも入れる。

カテゴリは2つにする。`<plan>` は「承認」のボタンを持ち、アプリを前面に出さない。`<done-merge>` は「マージ」のボタンを持ち、アプリを開く。ほかの通知にはカテゴリを付けない。

`expo-notifications` が、dataだけのメッセージをカテゴリ付きで表示できるかは、文書で確かめられていない。実機で確かめ、ROADMAPの検証項目に残す。表示できなければ、バックグラウンドの通知のタスク（`registerTaskAsync`）で受けて、`scheduleNotificationAsync` でローカルの通知として出す。

### D6. 承認は専用の口で、待ちの開始時刻を照らす（Q12、Q13、Q17）

承認の語を返答の口で送る形にせず、`<POST /v1/approve>`（WUIでは `<POST /api/approve>`）を足す。受けるのはrefと待ちの開始時刻だけで、投稿する文は受信箱のサービスが設定の `<plan.approve_word>` から決める。アプリが任意の文を送れる口を増やさずに済む。

待ちの投稿の識別子には、一覧の `waiting_since`（待ちの投稿のマーカーの `at`）を使う。計画を出し直せば新しい投稿になり、時刻が変わる。コメントのIDは今の一覧の項目に無く、足すとAPIの型が増えるので使わない。照らしてから投稿するまでの間に計画が出し直される可能性は残る。この間は、フォージの1往復の時間である。

アプリは、前面に出ないままボタンの処理を動かす。アプリが終了しているときに動くかは、実機で確かめる。動かなければ、`<plan>` のボタンを押したときにアプリを開き、開いた直後に承認を送る形へ変える（Q17）。

### D7. マージはWUIのダイアログに渡す（Q12のC2）

「マージ」のボタンは、`/issue/<ref>?action=merge` をWebViewで開く。WUIの1件の画面は、詳細を読んだあとで、問い合わせがあれば今の `MergeButton` のダイアログを開く。マージの検査（マーカーの `sha` との照合）は、今のWUIと受信箱のサービスのものをそのまま使う。

### D8. 通知を押したときの画面の移動

通知の本体を押すと、アプリは `/issue/<ref>` をWebViewで開く。アプリが終了していたときは、起動の処理の中で最後の通知の応答（`getLastNotificationResponseAsync`）を読み、WUIの画面の初めのURLにする。

### D9. 文書

- ADR-047を足す。内容は、WebViewの殻にしたこと、受信箱のサービスがFCMで送ること、Googleに渡す内容を参照、理由、タイトルに限ること、計画の「承認」のボタンを原則3の例外とすることである
- PRODUCT.mdのPlatformにAndroidのアプリを足し、原則3に例外を書く
- ARCHITECTURE、SECURITY（T5にアプリのPOST、T9に通知の内容）、README（Firebaseの準備、APKの作り方）を直す

## Risks / Trade-offs

- ［通知のタイトルがGoogleのサーバーを通る］→質問と計画の本文は載せない。Telegramでは本文の抜粋まで送っているので、外に出る量は今より少ない
- ［LANの中の別の機器がトークンを登録し、通知を受けられる］→ADR-036で受け入れた「LANの中なら一覧を読める」の範囲に収まる。宛先の上限を10にし、無効なトークンは消す
- ［全文を読まずに計画を承認できる］→利用者の判断（Q12）。理由が `plan` のときに限り、待ちの開始時刻で古い通知を断る
- ［`expo-notifications` の扱いが文書と違う（D5、D6）］→実機で確かめ、検証項目に残す。代わりの形を決めてある
- ［WUIに届かないとアプリは何もできない］→届かないことを出し、設定の画面へ移れるようにする。家の外からの利用は対象外
- ［ExpoのSDKの更新が速い］→SDK 58に固定し、ネイティブの依存を次の7つに限る
  - `react-native-webview`、`expo-notifications`、`expo-build-properties`、AsyncStorage
  - `expo-task-manager`：終了したアプリで「承認」のボタンを処理する（D6）
  - `react-native-safe-area-context`：画面の端まで描くAndroidで、余白を取る
  - `expo-system-ui`：ダークモードに従う
- ［targetSdk 37のSDKに上げると、LANへの通信に `ACCESS_LOCAL_NETWORK` の実行時の許可が要る］→上げるときの課題として残す（grilling.mdの後回し）
- ［画面が無くても一覧を取り直すので、フォージへの問い合わせが増える］→FCMを設定したときだけにする。間隔の下限10秒は変えない

## Migration Plan

設定に `<inbox.notify.fcm>` を書かなければ、受信箱のサービスとWUIの動きは変わらない。導入の順は次のとおり。

1. Firebaseのプロジェクトを作り、Androidのアプリを登録して `google-services.json` を得る。サービスアカウントの鍵をSecret Serviceに置く
2. 受信箱のサービスとWUIを入れ替え、`<inbox.notify.fcm>` を書いて受信箱のサービスを再起動する
3. `make app-apk` でAPKを作り、スマホに入れてURLを設定する

戻すときは、`<inbox.notify.fcm>` を消して再起動する。アプリは消してよい。`<push-devices.json>` と `<push-sent.json>` は残っても害が無い。
