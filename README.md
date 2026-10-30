# switchboard

GitHub/GitLabのissueを起点に、複数端末・複数プロダクトにまたがるClaude Codeの作業を
「応答待ち」単位で一覧し、その場から返答できるようにする個人用ツール。

電話交換台（switchboard）のように、散らばった回線（端末・リポジトリ・セッション）を
一か所で捌くことを目的とする。

## 何を解決するか

- Claude Codeのセッションが複数端末・複数リポジトリに散らばり、どれが自分の応答を待っているか分からない
- 各プロダクトのissueを毎回巡回して状況を確認したくない
- 応答を返すために、その端末・そのターミナルまで行きたくない

## 基本方針（要約）

| 方針 | 内容 |
|---|---|
| issue駆動 | 作業単位・状態・応答経路は、すべてissueで管理する |
| ローカルは外向き通信のみ | 端末側に受信ポートを開けない |
| 中継は呼び鈴だけ | 中継サーバーは「issue IDが更新された」ことだけを伝え、本文は必ずローカルがフォージから取り直す |
| 状態はissueとリポジトリに置く | 毎回まっさらな文脈で起動できる設計を基本とし、`--resume` は最適化扱い |
| 横断一覧はローカルで集約 | 両フォージのラベル検索結果をマージして表示する |

## ドキュメント

| ファイル | 内容 |
|---|---|
| [AGENTS.md](AGENTS.md) | Claude Code で本リポジトリを開発する際の指示 |
| [docs/SPEC.md](docs/SPEC.md) | 想定仕様（要件・状態モデル・CLI） |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | 構成・コンポーネント・処理フロー |
| [docs/DESIGN.md](docs/DESIGN.md) | システム設計（一部採用。ADR案・詳細設計・追加検証項目） |
| [docs/SECURITY.md](docs/SECURITY.md) | 脅威モデルと対策 |
| [docs/DECISIONS.md](docs/DECISIONS.md) | 設計判断の記録（ADR） |
| [docs/ROADMAP.md](docs/ROADMAP.md) | 段階的な実装計画と検証項目 |
| [docs/PRIOR_ART.md](docs/PRIOR_ART.md) | 調査した類似事例・OSS |
| [config/switchboard.example.yaml](config/switchboard.example.yaml) | 設定ファイル例 |

## ステータス

Phase 2の実装中。Phase 1は完了し、Ubuntuの端末と実物のGitHubで、着手から待ち、返答、draftのPRの作成まで回ることを確かめている（[docs/ROADMAP.md](docs/ROADMAP.md) の検証項目V20）。Phase 2では、GitLab（GitLab.comとセルフホスト）のプロジェクトも扱い、両フォージの待ちを1つの受信箱に出す。対象はLinux（Ubuntu）の1台の端末で、issueの更新はポーリングで検知する。中継サーバーと常駐化は、[docs/ROADMAP.md](docs/ROADMAP.md) の後のフェーズで扱う。

## 必要なもの

- Linux（Ubuntuで確認）
- Go 1.24以上と、git
- Claude Code CLI（`claude`）。2.1.312で確認した
- Secret Service（GNOME Keyringなど）と、`secret-tool`（Ubuntuでは `libsecret-tools` パッケージ）

## インストール

クローンから作り、`PATH` の通ったディレクトリに置く。

```bash
git clone https://github.com/uskanda/switchboard.git && cd switchboard
```

```bash
make build && install -D -m 0755 bin/switchboard ~/.local/bin/switchboard
```

mainブランチに取り込まれた版は、`go install` でも入れられる。

```bash
go install github.com/uskanda/switchboard/cmd/switchboard@latest
```

## トークンの登録

GitHubのfine-grained personal access tokenを2つ作り、Secret Serviceに登録する。設定ファイルには、トークンを書かない。

| 用途 | 設定のキー | 権限（対象のリポジトリだけ） |
|---|---|---|
| エージェント（ラベル、コメント、push、PR） | `token_ref` | Contents、Issues、Pull requestsの読み書き |
| 受信箱（一覧と返答） | `inbox_token_ref` | Issuesの読み書き |

```bash
secret-tool store --label='switchboard github' service switchboard/github username token
```

```bash
secret-tool store --label='switchboard github inbox' service switchboard/github-inbox username token
```

どちらのコマンドも、トークンを標準入力から読む。トークンは `claude` には渡さない。

GitLabのプロジェクトを使うときは、GitLabのpersonal access tokenを作り、同じように登録する。GitLab 19.2以降なら、fine-grainedのトークンで対象のプロジェクトと操作を絞れる。対象のプロジェクトを選び、次の権限を付ける。それより古いGitLabでは、`api` スコープの従来のトークンを使う。エージェント用と受信箱用に、同じトークンを登録してもよい。

| リソース | 権限 |
|---|---|
| Project | Read |
| Work Item | Read、Create、Update |
| Label | Read、Create |
| Merge Request | Read、Create |
| Code | Download、Push |

```bash
secret-tool store --label='switchboard gitlab' service switchboard/gitlab username token
```

```bash
secret-tool store --label='switchboard gitlab inbox' service switchboard/gitlab-inbox username token
```

## 設定

例をコピーして、`YOUR_GITHUB_USER`、端末の名前、プロジェクトのパスを書き換える。

```bash
mkdir -p ~/.config/switchboard
```

```bash
cp config/switchboard.example.yaml ~/.config/switchboard/config.yaml
```

書き換えたら、`doctor` で確かめる。トークン、ユーザーID、担当プロジェクトのリポジトリ、`claude` の版を順に調べ、問題があれば0以外の終了コードで終わる。

```bash
switchboard doctor
```

Phase 1では、エージェントの投稿も自分のアカウントで行う。対象は非公開リポジトリとし、`trusted_authors` には自分だけを書く。`claude` は自分のOSユーザーで動くため、詳しくは [docs/SECURITY.md](docs/SECURITY.md) を読むこと。

### GitLab

設定例のうち、字下げしたコメントの行がGitLabの設定である。コメントを外し、`forges` の `gitlab` と、`trusted_authors` の `gitlab` を書く。`base_url` を省くと `https://gitlab.com` を使う。プロジェクトは `gl:group/project` の形で書き、サブグループを含む `gl:group/sub/project` も書ける。

GitLabの `internal` のプロジェクトは、インスタンスの全ユーザーが見てコメントできるため、公開のプロジェクトとして扱う。`doctor` は警告を出し、`include_untrusted: true` を拒否する。

### プロジェクトごとのエージェントの身元

エージェントの投稿とpushを1つのプロジェクトに限りたいときは、そのプロジェクトにProject Access Token（GitLab）か、そのリポジトリだけのトークンを作る。プロジェクトの設定に `token_ref` と `agent_identity` を組で書くと、そのプロジェクトのエージェントだけがそのトークンとbotの身元を使う。受信箱は、上書きにかかわらず、フォージの `inbox_token_ref` を使う。

```yaml
projects:
  - ref: gl:YOUR_GROUP/switchboard-sandbox
    path: ~/repos/switchboard-sandbox-gl
    host: ubuntu-a
    token_ref: switchboard/gitlab-sandbox-bot
    agent_identity: project_N_bot_XXXX   # the bot user of the project access token
```

## 使い方

対象のリポジトリに、状態ラベル（`agent:ready`、`agent:running`、`agent:waiting`、`agent:done`）を作る。

```bash
switchboard labels sync
```

担当プロジェクトの端末で、エージェントを動かす。Ctrl-Cで止まる。

```bash
switchboard agent run
```

issueに `agent:ready` を付けると、次のポーリングで `agent:running` になり、issueごとのworktreeで `claude -p` が動く。実行の終わりは2通りである。

- 質問を残したとき：質問のコメントが付き、`agent:waiting` になる。issueに返信すると、次のポーリングで再び実行する
- 完了したとき：`verify_command` と出力の検査を通れば、`switchboard/issue-<n>` ブランチをpushしてdraftのPRを作り、`agent:done` になる

別の端末から、エージェントの状態を見る。

```bash
switchboard agent status
```

全プロジェクトの待ちのissueを、GitHubとGitLabをまとめて、待ち始めた順に一覧する。`--json` を付けると、JSONの配列で出る。`--forge github` か `--forge gitlab` で、片方のフォージに絞れる。片方のフォージに届かないときは、届いた側の一覧を出したうえで、0以外の終了コードで終わる。

```bash
switchboard inbox
```

一覧の行番号で、質問の全文を読み、返答する。`--editor` を付けると、`$EDITOR` で返答を書ける。

```bash
switchboard show 1
```

```bash
switchboard reply 1 "Bで進めてください"
```

すべてのコマンドは `--config` で設定ファイルを指定できる。指定が無いときは `~/.config/switchboard/config.yaml` を読む。

## 開発

```bash
make verify
```

実物の `claude` で、着手から質問、返答、完了までを通す確認もある。モデルを呼ぶため、`make verify` とCIでは動かない。

```bash
go test -tags realclaude -run RealClaude -v -timeout 30m ./internal/integration/
```

Secret Serviceが無い開発環境では、設定に `secrets: {allow_env: true}` を書く。するとSecret Serviceに項目が無いとき、環境変数からトークンを読む。変数の名前は、`token_ref` が `switchboard/github` なら `SWITCHBOARD_TOKEN_SWITCHBOARD_GITHUB` である。

## ライセンス

MIT。[LICENSE](LICENSE) を参照。
