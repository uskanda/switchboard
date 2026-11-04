# switchboard

GitHubとGitLabのissueを起点に、Claude Codeの自動の実行を複数の端末とリポジトリで動かす個人用ツール。エージェントが質問を残して止まったissueを「待ち」として1つの受信箱に集め、CLI、ブラウザ、Androidのアプリから返答できる。

紹介ページは <https://uskanda.github.io/switchboard/> にある。

## 何を解決するか

- Claude Codeのセッションが複数の端末とリポジトリに散らばり、どれが自分の返答を待っているか分からない
- 各プロダクトのissueを巡回して、状況を確かめたくない
- 返答するために、その端末のターミナルまで行きたくない

## 動き方

1. issueに `agent:ready`（計画から始めるなら `agent:plan`）を付ける
2. そのプロジェクトを担当する端末のエージェントが、issueごとのworktreeで `claude -p` を動かす
3. 実行が質問を残したら、issueに質問のコメントが付き、`agent:waiting` になる。待ちはTelegramとAndroidのアプリに通知される
4. 受信箱（`switchboard inbox`、Web表示、アプリ）から返答すると、エージェントが同じissueで実行を再開する
5. 完了すると、検証のコマンドを通したブランチをpushし、draftのPR/MRを作って `agent:done` にする。受信箱からマージできる

CIが失敗したとき、pushが拒否されたとき、baseと衝突したときは、エージェントが直す実行を自動で始める。

## 基本方針

| 方針 | 内容 |
|---|---|
| issue駆動 | 作業の単位、状態、返答の経路は、すべてissueで管理する |
| ローカルは外向きの通信だけ | 端末に受信ポートを開けない。例外は受信箱のWeb表示で、LANのアドレスに限る（ADR-036） |
| 中継は呼び鈴だけ | 中継はissueのIDだけを伝える。本文は、ローカルが必ずフォージから取り直す |
| issueの文字列はデータ | 指示として扱うのは `trusted_authors` のコメントだけである |
| `claude` に資格情報を渡さない | push、PRの作成、コメントはエージェントが行う。実行はsandboxの中で動き、ネットワークに出られない |
| 横断の一覧はローカルで集める | 両フォージのラベルの検索の結果をまとめて出す |

## 主な機能

- 計画を先に立てるモード：実装の前に質問と計画をコメントし、`/ok` の返答で承認してから実装する
- 複数の端末とプロジェクト：プロジェクトごとに担当の端末を決める。2台が同じissueを重ねて実行しない
- 受信箱：CLI、LAN向けのWeb表示（`switchboard-web`）、Androidのアプリ（`app/`）。Web表示からissueを作り、プロジェクトを足し、PR/MRをマージできる
- 通知：Telegram（エージェントが送る）と、FCMによるAndroidのプッシュ通知（受信箱のサービスが送る）
- 中継（`switchboard relay serve`）：GitLabのwebhookを受け、返答から数秒で実行を始める
- 常駐：systemdのサービスとして置き、トークンは暗号化したsystemdのクレデンシャルで渡す
- 実行の制御：同時に動かす数の上限、順番待ち、利用上限での一時停止、APIの費用の上限、モデルとeffortの指定、実行への追加の指示と許可

## ステータス

Phase 1からPhase 5の機能を実装し、作者の端末（Ubuntu 24.04）と、実物のGitHubとGitLab（セルフホストのCE）で動かしている。2台の端末での実物の確認と、いくつかのフォージのAPIの挙動は、まだ確かめていない。未確認の項目は [docs/ROADMAP.md](docs/ROADMAP.md) の検証項目にある。

## 前提と制約

作者1人の環境で作り、確かめてきた。ほかの人が使うときは、次の前提に合うかを先に確かめること。

- 利用者は1人である。`trusted_authors` に自分だけを書く。エージェントも、既定では自分のアカウントで投稿する。公開リポジトリも扱えるが、そのときは `include_untrusted` を使えず、指示として採るのは `trusted_authors` の入力だけである（ADR-060）
- 端末はLinuxで、systemd、Secret Service（GNOME Keyringなど）、`bubblewrap`、`socat` が要る。macOSでは動作を確かめていない
- 受信箱のWeb表示は利用者を認証しない。LANの中の機器なら誰でも返答し、issueを作れる
- 中継は `127.0.0.1` で待ち受ける。GitHub.comのwebhookは届かないので、GitHubのプロジェクトはポーリングで扱う
- GitHubはgithub.comだけを扱う。GitHub Enterprise Serverには対応していない
- 配布するバイナリとAPKは無い。ソースから作り、Androidのアプリは自分のFirebaseのプロジェクトと署名の鍵で作る
- Web表示とアプリの文言は日本語だけである。issueに書くコメントは `comments.language` で英語と日本語を選べる
- サブスクリプションの認証で無人の実行を常駐させてよいかは、switchboardでは確かめていない。Claudeの利用規約に照らして、利用者が判断する

## 必要なもの

- Linux（Ubuntu 24.04で確認）とsystemd
- Go 1.24以上と、git
- Node.js 24とpnpm 12（Web表示を作るため）
- Claude Code CLI（`claude`）。`switchboard doctor` が、確かめた版より古いと警告する
- Secret Serviceと `secret-tool`（Ubuntuでは `libsecret-tools` パッケージ）
- `bubblewrap` と `socat`（実行のsandbox）
- GitHubのプロジェクトで完了のスクリーンショットを付けるときは、`gh` v2.99.0以上

## はじめ方

クローンから作り、`~/.local/bin` に置く。

```bash
git clone https://github.com/uskanda/switchboard.git && cd switchboard
```

```bash
make install
```

フォージのトークンを作ってSecret Serviceに置く。権限の一覧は [docs/GUIDE.md](docs/GUIDE.md#トークンの登録) にある。トークンは標準入力から読む。

```bash
secret-tool store --label='switchboard github' service switchboard/github username token
```

```bash
secret-tool store --label='switchboard github inbox' service switchboard/github-inbox username token
```

設定の例をコピーし、`YOUR_GITHUB_USER`、端末の名前、プロジェクトのパスを書き換える。

```bash
mkdir -p ~/.config/switchboard && cp config/switchboard.example.yaml ~/.config/switchboard/config.yaml
```

設定、トークン、`claude` の版を確かめてから、対象のリポジトリに状態ラベルを作る。

```bash
switchboard doctor
```

```bash
switchboard labels sync
```

エージェントを動かし、issueに `agent:ready` を付ける。待ちは受信箱で見て返答する。

```bash
switchboard agent run
```

```bash
switchboard inbox
```

```bash
switchboard reply 1 "Bで進めてください"
```

常駐、中継、通知、Web表示、Androidのアプリの設置は [docs/GUIDE.md](docs/GUIDE.md) にある。

## ドキュメント

| ファイル | 内容 |
|---|---|
| [docs/GUIDE.md](docs/GUIDE.md) | 運用ガイド（トークン、設定、各機能の使い方と設置） |
| [docs/SPEC.md](docs/SPEC.md) | 想定仕様（要件、状態モデル、CLI） |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | 構成、コンポーネント、処理の流れ |
| [docs/DESIGN.md](docs/DESIGN.md) | システム設計（ADRの案、詳細設計、追加の検証項目） |
| [docs/SECURITY.md](docs/SECURITY.md) | 脅威モデルと対策 |
| [docs/DECISIONS.md](docs/DECISIONS.md) | 設計判断の記録（ADR） |
| [docs/ROADMAP.md](docs/ROADMAP.md) | 段階的な実装計画と検証項目 |
| [docs/PRIOR_ART.md](docs/PRIOR_ART.md) | 調べた類似の事例とOSS |
| [docs/examples/](docs/examples/) | 実行への指示の例（OpenSpec、スクリーンショット） |
| [config/switchboard.example.yaml](config/switchboard.example.yaml) | 設定ファイルの例 |
| [AGENTS.md](AGENTS.md) | 本リポジトリをClaude Codeで開発するときの指示 |

## 開発

```bash
make verify
```

`make verify` は、Goの検査とテストに加えて、`web/` と `app/` の検査も実行する。AndroidのAPKを作る `make app-apk` だけは、JDKとAndroid SDKも要る。画面の見た目の基準は [DESIGN.md](DESIGN.md)、使う人と目的は [PRODUCT.md](PRODUCT.md) にある。紹介ページは `site/` にあり、mainに入るとGitHub Pagesに公開される。

実物の `claude` で、着手から質問、返答、完了までを通す確認もある。モデルを呼ぶため、`make verify` とCIでは動かない。

```bash
go test -tags realclaude -run RealClaude -v -timeout 30m ./internal/integration/
```

Secret Serviceが無い開発環境では、設定に `secrets: {allow_env: true}` を書く。するとSecret Serviceに項目が無いとき、環境変数からトークンを読む。変数の名前は、`token_ref` が `switchboard/github` なら `SWITCHBOARD_TOKEN_SWITCHBOARD_GITHUB` である。

## ライセンス

MIT。[LICENSE](LICENSE) を参照。
