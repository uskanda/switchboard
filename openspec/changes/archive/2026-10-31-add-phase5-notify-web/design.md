# Design

## Context

動機と範囲はproposal.mdに、判断はgrilling.mdにある。設計の前提になる今の形は、次のとおりである。

- エージェントは、`writeBack` で結果のコメントを投稿し、ラベルを替える。既に同じ実行のコメントがあれば投稿しない（二重投稿の防止）。投稿したときだけ、監査ログに `comment.posted` を残す。完了のときは、`outcome.PR` に作ったPRかMRのURLが入る
- `forge.IssueSummary` は、issueのURLを持たない。GitHubのAPIは `html_url` を、GitLabのAPIは `web_url` を返す
- `internal/inbox` の `List` は、届かなかったフォージの誤りを `errors.Join` でまとめて返し、行番号をキャッシュに書く。`Show` は `io.Writer` に直接書く。`Reply` は、投稿前の確認を済ませてから投稿する
- `make verify` は、gofmt、`go vet ./...`、`go test ./...` を実行する。CIは、Goだけを用意して `make verify` を実行する。開発の端末には、Node.js 24とpnpm 12がある
- usknasでは、Telegramのbotとchat idが、別の用途の通知で使われている（`~/.config/.telegram-token`）

## Goals / Non-Goals

**Goals:**

- 通知は、エージェントの処理の成否に影響しない
- 受信箱のトークンを持つプロセスは、核の `switchboard inbox serve` だけにする。WUIのプロセスが侵害されても、トークンは取られない
- WUIは、核の `internal/` を読まない。核とWUIの契約は、`<inboxapi>` の型だけにする

**Non-Goals:**

- WUIの認証、HTTPS、家の外からの利用
- 画面の自動更新と、GitHubの受信箱のGraphQLへの切り替え（V14）
- WUIからのラベルの操作や、スレッド全体の表示

## Decisions

### 1. 通知（`internal/notify`）

- `Telegram{Token, ChatID, BaseURL, HTTP}` の `Send(ctx, text)` は、`chat_id` と `text` をJSONで送る。送り先は `POST <BaseURL>/bot<token>/sendMessage` である。`parse_mode` は付けず、平文として送る。`BaseURL` の既定は `https://api.telegram.org` で、テストでは `httptest` に差し替える
- Goの `*url.Error` は、URLを誤りの文に含める。URLにはトークンが入るので、`Send` は誤りを、HTTPの状態か、トークンを含まない短い理由に包み直して返す。テストで、誤りの文にトークンが現れないことを確かめる
- 1回の送信は、`context.WithTimeout` で10秒に打ち切る
- エージェントは `Notifier` インターフェースを持ち、`writeBack` が実際に投稿したときだけ呼ぶ。二重投稿の防止で投稿しなかったときは呼ばない。失敗したら `<notify.failed>`（issue、kind、reason）を監査ログに残し、`writeBack` の戻り値には影響させない
- 通知は、ラベルを替えたあとに送る。送るのに時間がかかっても、ラベルが遅れないようにするためである

通知の本文は次の形にする。質問の抜粋は、`inbox.Excerpt` と同じ規則で取り、300文字で切る。全体はTelegramの上限（4,096文字）より十分短い。

```
⏸ 待ち（question）gl:nas/switchboard-sandbox#6
Add a relay note
質問：Which word should I use: fast or quick?
https://gl.uskan.dev/nas/switchboard-sandbox/-/issues/6
受信箱：http://192.168.0.82:8783/issue/gl:nas/switchboard-sandbox%236
```

```
✅ 完了 gh:uskanda/switchboard-sandbox#5
PR：https://github.com/uskanda/switchboard-sandbox/pull/6
https://github.com/uskanda/switchboard-sandbox/issues/5
```

issueのURLを得るために、`forge.IssueSummary` に `URL` を足し、GitHubとGitLabのクライアントで `html_url` と `web_url` から埋める。偽物のフォージも、決まった形のURLを返す。

### 2. 受信箱の分け直し（`internal/inbox`）

APIのサーバーから使えるように、次のように分ける。CLIの表示は今までどおりにする。

- `Collect` を足す。`[]Item` と、フォージごとの誤り（`[]ForgeError{Forge, Err}`）を返し、行番号を記録しない。`List` は、`Collect` を呼んで行番号を記録する形にし、CLIから今までどおり使う。APIは行番号を使わない
- `Show` は、`Detail` を返す `Detail` と、それを書く `PrintDetail` に分ける。`Detail` は、タイトル、URL、待ちかどうか、マーカーの項目、エージェントの最新の投稿（質問と拒否された操作を含む）を持つ
- `Reply` は今のままにする。誤りは、`errors.Is` で `ErrInvalid`、`ErrNotFound`、`ErrNotWaiting` を区別できるようにする。行番号を受けない `ParseRef` も足す

### 3. 受信箱のAPI（`switchboard inbox serve`、`<inboxapi>`）

- 公開のパッケージは、リポジトリ直下の `inboxapi/` に置く。中身は、要求と応答の型、誤りの型、`Client` である。`Client` は、Unixソケットに `net.Dial("unix", path)` でつなぐ `http.Client` を持つ
- サーバーは `internal/inboxserve` に置き、`inboxapi` の型で応答する。経路は次の3つで、`/v1` を付けて、将来の互換性の破れに備える

| 経路 | 内容 |
|---|---|
| `GET /v1/inbox` | `{"items": [...], "errors": [{"forge", "message"}], "fetched_at"}` |
| `GET /v1/issue?ref=<ref>` | `Detail` |
| `POST /v1/reply` | 要求は `{"ref", "text", "force"}`。成功なら204 |

- 誤りは、`{"code", "message"}` で返す。`code` は、`not_waiting`（409）、`invalid`（400）、`not_found`（404）、`forge`（502）である。`Client` は、`not_waiting` を `<inboxapi.ErrNotWaiting>` に直す
- 一覧のキャッシュは、取得の時刻と結果をミューテックスで守って持つ。60秒以内ならフォージに問い合わせない。返答に成功したら捨てる
- ソケットは、起動のときに古いファイルがあれば消してから作り、`chmod 0600` する。親のディレクトリが無ければ0700で作る。ソケットのファイルの権限が、同じユーザーのプロセスだけに呼ばせる唯一の仕組みである。それ以上の認証はしない
- `--socket` の既定は、`$RUNTIME_DIRECTORY/inbox.sock`、無ければ `$XDG_RUNTIME_DIR/switchboard/inbox.sock` とする。どちらも無ければ起動を拒む

CLIを呼ぶ案と、gRPCかConnectの案は退けた（grilling.md）。HTTPとJSONなら、標準ライブラリだけで書け、`curl --unix-socket` で確かめられる。

### 4. WUIのモジュール（`web/`）

```
web/
  go.mod                 module github.com/uskanda/switchboard/web
                         require github.com/uskanda/switchboard（replace ../）
  cmd/switchboard-web/   serve サブコマンド
  internal/server/       APIの中継、Origin と Host の検査、静的ファイル
  internal/assets/       go:embed all:dist（dist/ はビルドで作る。.gitkeep だけをコミット）
  ui/                    Vite、React、TypeScript、Tailwind、shadcn/ui（pnpm）
```

- バックエンドの経路は、`GET /api/inbox`、`GET /api/issue?ref=`、`POST /api/reply` である。それぞれ `<inboxapi>` を呼び、結果をそのまま返す。それ以外の経路は `index.html` を返し、画面の振り分けはフロントエンドで行う
- `dist/` が空のまま起動したときは、フロントエンドがビルドされていないことを `/` に表示する。`go vet` と `go test` は、フロントエンドをビルドしなくても通る
- 返答のPOSTでは、まず `Content-Type: application/json` を要求する。そのうえで、`Origin` が `http://<listen>` と、`Host` が `<listen>` と一致することを確かめる。JSONを要求すると、別のオリジンからは、ブラウザの事前の確認（preflight）無しに送れなくなる。WUIはCORSのヘッダーを返さないので、事前の確認は通らない
- 応答には、`Content-Security-Policy: default-src 'self'` と `X-Content-Type-Options: nosniff` を付ける。Reactは文字列をテキストとして描くので、issueのタイトルと質問をHTMLとして解釈させない。`dangerouslySetInnerHTML` は使わない

### 5. 画面（`web/ui`）

- 画面は、一覧（`/`）と1件（`/issue/<ref>`）の2つである。振り分けにはReact Routerを使う
- 一覧は、幅が広ければ表、狭ければカードの並びで出す。項目は、プロジェクトとissue番号、タイトル、端末、待ちの経過時間、理由、質問の冒頭である。届かなかったフォージは、一覧の上に警告として出す
- 1件の画面は、質問の全文、拒否された操作、フォージへのリンク、返答の欄、送るボタンを出す。待ちでないと返されたら、その旨を出し、`--force` に当たる操作は出さない
- 部品は、shadcn/uiのButton、Card、Badge、Textarea、Alert、Tableを取り込む。文言は日本語にする
- 見た目の基準は、実装の最初に、`ui-guidelines` に従ってリポジトリ直下の `<DESIGN.md>` と `<PRODUCT.md>` に書く
- テストはVitestとTesting Libraryで書く。型の検査は `tsc --noEmit`、lintはESLintで行う

### 6. WUIの設定（`<web.yaml>`）

```yaml
listen: 192.168.0.82:8783
socket: /run/switchboard/inbox.sock   # 省略したら核と同じ決め方
```

`listen` のホストは、IPアドレスに限る。`0.0.0.0`、`::`、空を拒む。`127.0.0.1` は、開発のために許す。ホスト名を許さないのは、`Host` の検査を、決まった1つの値との比較にするためである。

### 7. サービス（`internal/service`）

- `<switchboard-inbox.service>` の `ExecStart` は、`switchboard inbox serve --config <path>` とする。unitには `RuntimeDirectory=switchboard` と `RuntimeDirectoryMode=0700` を指定する。クレデンシャルは各フォージの `inbox_token_ref` である
- `<switchboard-web.service>` は、`After=` と `Wants=` で受信箱のサービスの後に起こす。`ExecStart` は、`<switchboard-web の絶対パス> serve --config ~/.config/switchboard/web.yaml` である。`RuntimeDirectory=` は指定しない。systemdは、unitが止まると、そのunitの `RuntimeDirectory` を消す。2つのunitで同じディレクトリを指定すると、WUIを止めたときに受信箱のソケットまで消えてしまう。WUIは同じ `User=` で動くので、受信箱のサービスが作ったディレクトリを開ける。grilling.mdのQ24では「両方のunitに指定する」としたが、この理由で、受信箱のサービスだけに指定する
- エージェントのサービスでは、`<notify.telegram>` の `token_ref` もクレデンシャルにする
- `--inbox` と `--web` は、`--relay` と同じく `Setup` の種類として足す。sudoの前の確認と、表示するコマンドの一覧は、今の仕組みをそのまま使う

### 8. 検証の入口（`Makefile`、CI）

- `make verify` に `verify-web` を足す。`web/` では、次の順に実行する
  1. `pnpm install --frozen-lockfile`
  2. `pnpm typecheck`、`pnpm lint`、`pnpm test`、`pnpm build`
  3. `go vet ./...` と `go test ./...`
- 直下の `go vet ./...` は、別の `go.mod` を持つ `web/` を含まない。gofmtの検査は、`git ls-files` で `web/` のGoのファイルも含める
- CIに、`pnpm/action-setup` と `actions/setup-node` を足す
- `make build` は、`bin/switchboard` と `bin/switchboard-web` を作る

### 9. 文書

- ADR-036：WUIに限って、端末がLANのアドレスで受信ポートを開くことを許す。当面は認証せず、返答のPOSTでは `Origin` と `Host` を検査する。AGENTS.mdとSECURITY.mdのT5の不変条件を、この例外に合わせて書き直す
- ADR-037：ADR-010（1つのバイナリ）の範囲を核に限る。WUIは別のモジュールとし、フロントエンドの構成をVite、React、TypeScript、shadcn/uiとする。核とWUIは `<inboxapi>` だけでつなぐ
- READMEに、Telegramのトークンの置き方、`inbox serve` とWUIの設置、`<web.yaml>` の書き方を書く
- WUIの認証を課題としたGitHubのissueを作る。LANの中の別の機器から、`uskanda` として返答できてしまうことを書く

## Risks / Trade-offs

- ［WUIに認証が無いので、LANの中の機器から `uskanda` として返答できる。その返答は、信頼された指示としてエージェントに渡る］→利用者が受け入れたリスクである。ADR-036とissueに記録する。外のWebページからの返答は、`Origin` と `Host` の検査で防ぐ
- ［ホスト名でWUIを開くと、`Host` の検査で返答が拒まれる］→`listen` にはIPアドレスを書かせ、通知のリンク（`<notify.web_url>`）にも同じアドレスを書く。READMEに書く
- ［Telegramに、issueのタイトルと質問の抜粋が渡る］→利用者が選んだ内容である（grilling.mdのQ4'）。トークンとシークレットは、出力の検査を通った投稿からしか取らない
- ［トークンが誤りの文からログに漏れる］→`Send` が誤りを包み直す。テストで確かめる
- ［npmの依存が増える］→ロックファイルを固定し、`--frozen-lockfile` で入れる。依存は、Vite、React、React Router、Tailwind、shadcn/uiの部品、テストの道具に絞る
- ［usknasの防火壁が、WUIのポートを塞いでいる］→実物で確かめるときに見る。開ける必要があれば、sudoのコマンドを示してから利用者に頼む

## Migration Plan

- `<notify>` の無い設定と、`--inbox` と `--web` を使わない設置は、今までどおり動く
- 使い始めるときは、次の順に行う
  1. Telegramのbotのトークンを、Secret Serviceに置く
  2. `<notify.telegram>` を設定に足し、`service install` でエージェントを置き直す
  3. `service install --inbox` と `service install --web` を実行する
- 戻すときは、`service uninstall --web` と `service uninstall --inbox` を実行する。そのうえで、設定から `<notify>` を消して `service install` をやり直す
