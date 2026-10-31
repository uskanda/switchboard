# Proposal

## Why

Phase 4までで、エージェントは常駐し、返答からすぐに実行を再開できるようになった。しかし、issueが待ちになったと知るには、`switchboard inbox` を自分で実行するか、フォージの画面を見に行くしかない。返答も、CLIのある端末からしかできない。ROADMAP.mdのPhase 5は、待ちが起きたら知らせることと、スマホからも受信箱を見られるようにすることを挙げている。

## What Changes

- 待ちになったときと完了したときに、エージェントがTelegramに通知する
  - 待ちの通知には、プロジェクト、issue番号、タイトル、待ちの理由、質問の抜粋、issueのURLを載せる
  - 完了の通知には、プロジェクト、issue番号、作ったPRかMRのURL、issueのURLを載せる
  - 設定にWUIのURLがあれば、WUIへのリンクも足す
  - 送れなくても実行は止めず、監査ログに残すだけにする
- 核に `switchboard inbox serve` を足す。受信箱のトークンを持って常駐するプロセスである。UnixソケットでHTTPとJSONのAPI（一覧、1件、返答）を出す。一覧は60秒キャッシュする。核のモジュールに、公開のクライアントのパッケージ `<inboxapi>` を作る
- 同じリポジトリの `web/` に、受信箱のWeb表示（WUI）を別のモジュールとして作る
  - フロントエンドは、Vite、React、TypeScript、shadcn/uiで作る。成果物は、静的ファイルを埋め込んだ別のバイナリ `<switchboard-web>` とする
  - WUIのバックエンドは `<inboxapi>` で核を呼び、受信箱のトークンを持たない
  - 画面は、デスクトップとスマホの両方で使える
- WUIは、`<web.yaml>` に明示したLANのアドレスで待つ。`0.0.0.0` は拒否する。当面は認証を付けないが、返答のPOSTでは `Origin` と `Host` を検査する。認証は課題としてissueに残す
- `service install` に `--inbox` と `--web` を足す。それぞれ `<switchboard-inbox.service>` と `<switchboard-web.service>` を置く。エージェントのサービスには、Telegramのトークンもクレデンシャルとして渡す
- DECISIONS.mdに、ADRを2つ足す
  - ADR-036：WUIに限って、端末がLANで受信ポートを開くこと
  - ADR-037：ADR-010の範囲を核に限り、WUIを別のモジュールとし、フロントエンドの構成を決めること
- リポジトリ直下に、UIの基準として `<DESIGN.md>` と `<PRODUCT.md>` を作る。READMEに、通知とWUIの設置を書く

## Capabilities

### New Capabilities

- `notify`: 待ちと完了のTelegramへの通知、載せる内容、失敗したときの扱い
- `inbox-api`: `switchboard inbox serve` のUnixソケットのAPIと、公開のクライアント
- `web`: WUIの待ち受け、画面、返答、`Origin` と `Host` の検査

### Modified Capabilities

- `config`: 通知の設定（`<notify.telegram>`、`<notify.web_url>`）
- `service`: `--inbox` と `--web` のサービス、エージェントのサービスのTelegramのトークン

## Impact

- 新しく `internal/notify`（Telegramへの送信）と、`inbox serve` のサーバーを作る。公開のパッケージ `<inboxapi>` を足す
- `internal/agent` の書き戻しで、待ちと完了のときに通知を呼ぶ。`internal/inbox` を、APIのサーバーから呼べる形に分ける
- `internal/config` に `<notify>` 節を、`internal/service` に2つのunitとクレデンシャルを足す
- 新しく `web/` を作る。Go（独自の `go.mod`）と、Node.jsとpnpmの依存（Vite、React、TypeScript、Tailwind、shadcn/ui）が入る。`make verify` とCIに、`web/` の型の検査、lint、テスト、ビルドを足す
- Telegramのbotのトークンを、Secret Serviceの `switchboard/telegram` に置く必要がある。botとchat idは、usknasで運用中のものを使える
- README.md、DECISIONS.md、ROADMAP.md、SECURITY.md、AGENTS.md（受信ポートの不変条件）を更新する
