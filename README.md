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
| ローカルは外向き通信のみ | 端末側に受信ポートを開けない。例外は受信箱のWeb表示で、LANのアドレスに限る |
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

Phase 5の実装中。Phase 1からPhase 4までの実装を終えた。Phase 3の2台での実物の確認は、後回しにしている。GitHubとGitLab（GitLab.comとセルフホスト）のプロジェクトを扱い、両フォージの待ちを1つの受信箱に出せることを、実物で確かめた（[docs/ROADMAP.md](docs/ROADMAP.md) の検証項目V20、V24）。エージェントはsystemdのサービスとして常駐でき、複数の端末が同じissueを重ねて実行しない。Phase 4で、webhookを受ける中継を足し、返答から実行の開始までをポーリングの間隔に依存させなくした。Phase 5では、待ちと完了をTelegramに知らせ、ブラウザから受信箱を見て返答できるようにする。対象はLinux（Ubuntu）で、macOSは後のフェーズで扱う。

## 必要なもの

- Linux（Ubuntuで確認）
- Go 1.24以上と、git
- 受信箱のWeb表示（`switchboard-web`）を作るときは、Node.js 24とpnpm 12
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

`make build` は、受信箱のWeb表示の `bin/switchboard-web` も作る。使うときは、同じように置く。

```bash
install -D -m 0755 bin/switchboard-web ~/.local/bin/switchboard-web
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

GitLabのプロジェクトを使うときは、GitLabのpersonal access tokenを `api` スコープで作り、同じように登録する。エージェント用と受信箱用に、同じトークンを登録してもよい。このトークンは自分の全プロジェクトを操作できる。エージェントの影響を1つのプロジェクトに限りたいときは、後述のプロジェクトごとの上書きで、Project Access Tokenを使う。GitLab 19.2以降のfine-grainedのトークンでも絞れるが、switchboardが使うすべてのAPIで動くかは確かめていない。

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

### 常駐させる

ログインしていなくてもエージェントが動き続けるように、systemdのサービスとして常駐させられる。サービスは自分のOSユーザーで動き、異常に終わると自動で再起動する。トークンは、ホストの鍵で暗号化したsystemdのクレデンシャルとして置く。置くのは、この端末の担当プロジェクトのエージェント用トークンだけで、受信箱のトークンは置かない。

設置するときは、Secret Serviceからトークンを読むので、keyringを解錠しておく。まず、何をsudoで実行するかを確かめる。

```bash
switchboard service install --dry-run
```

表示されるのは、次のコマンドである（クレデンシャルの行は、トークンの数だけ並ぶ）。トークンは標準入力で渡し、画面には出さない。

| コマンド | 目的 |
|---|---|
| `sudo install -d -m 0700 /etc/credstore.encrypted` | クレデンシャルの置き場所を作る |
| `sudo systemd-creds encrypt --name=<名前> - /etc/credstore.encrypted/switchboard.<名前>` | トークンを暗号化して置く |
| `sudo install -m 0644 <一時ファイル> /etc/systemd/system/switchboard-agent.service` | unitを置く |
| `sudo systemctl daemon-reload` | unitを読み込ませる |
| `sudo systemctl enable switchboard-agent.service` | 起動時に動くようにする |
| `sudo systemctl restart switchboard-agent.service` | 動かし直す（初めてなら起動する） |

よければ設置する。コマンドを全部表示してから、`[y/N]` で確認を求める。sudoのパスワードは、端末で入力する。

```bash
switchboard service install
```

状態は `switchboard service status` で、ログは `journalctl -u switchboard-agent.service` で見る。撤去は `switchboard service uninstall` で行う。

次のときは、`switchboard service install` をもう一度実行する。

- トークンを入れ替えたとき（クレデンシャルを置き直す）
- `claude` や `git` の場所を変えたとき（unitには、設置したときの `PATH` を書く）
- 設定ファイルの場所や、担当のプロジェクトを変えたとき

サブスクリプションの認証（`claude.auth: subscription`）で常駐させてよいかは、switchboardでは確かめていない。Claudeの利用規約に照らして、利用者が判断すること。

### 複数の端末で動かす

担当は、プロジェクトの `host` で端末ごとに分ける。設定を誤って2台が同じプロジェクトを担当しても、同じissueを2台が実行することは無い。エージェントは、着手のときに「端末ubuntu-aで実行を始めます」のような着手のコメントを投稿してから、issueを取り直す。最も早く着手のコメントを出した端末だけが実行し、他の端末は自分のコメントを消して退く。2台とも、この版のswitchboardにすること。

### 中継で返答にすぐ着手する

既定では、エージェントはポーリングで更新を見つける。返答してから実行が始まるまで、最大で `poll_interval` だけかかる。中継（`switchboard relay serve`）を置くと、フォージのwebhookを受けた中継がissueのIDだけをエージェントへ配り、エージェントは数秒で取り直す。中継は、issueの本文、コメント、タイトル、ユーザー名を持たず、フォージのトークンも持たない。

今の版の中継は、受け口を `127.0.0.1` で待ち、公開しない。同じ端末で動くGitLabのプロジェクトに使う。GitHub.comからのwebhookには公開の受け口が要るので、GitHubのプロジェクトは当面ポーリングで扱う。

中継につながっている間、エージェントは `reconcile_interval`（既定10分）ごとに全プロジェクトを照合する。中継が止まったら、すぐに `poll_interval` のポーリングに戻り、中継が戻ったらつなぎ直す。

設置の手順は次のとおりである。

1. 端末のトークンを作る。トークンはSecret Serviceに置かれ、画面には、設定に足す項目（トークンのハッシュと、エージェントの `relay`）だけが出る

   ```bash
   switchboard relay token add <端末名>
   ```

2. GitLabの管理画面で、`Admin > Settings > Network > Outbound requests` を開く。「Local IP addresses and domain names that hooks and integrations can access」に、`127.0.0.1:8781` を足す。「Allow requests to the local network from webhooks and integrations」はオフのままにする。足してから数分の間、GitLabのバックグラウンドの処理は、このアドレスへの送信を拒むことがある
3. プロジェクトのwebhookを登録する。シークレットはSecret Serviceに置かれ、受信箱のトークンでwebhookが登録される。GitLabでは、署名トークンで登録する。トークンに権限が無ければ、画面から登録する手順が、シークレットとともに1回だけ表示される

   ```bash
   switchboard relay hook add gl:nas/switchboard-sandbox
   ```

4. 1と3で表示された項目を、設定ファイルの `relay:` 節と `agent:` 節に貼り付ける。switchboardは設定ファイルを書き換えない

   ```yaml
   relay:
     hooks:
       - project: gl:nas/switchboard-sandbox
         key: <hook-key>
         secret_ref: switchboard/relay-hook-<hook-key>
     hosts:
       - name: <端末名>
         token_hash: sha256:<16進>
   agent:
     reconcile_interval: 10m
     relay:
       url: http://127.0.0.1:8782/events
       token_ref: switchboard/relay-<端末名>
   ```

5. 中継をサービスとして置き、エージェントを中継モードで置き直す。sudoで実行するコマンドは、エージェントのサービスと同じ形で、unitの名前が `switchboard-relay.service` になる。中継のクレデンシャルは、webhookのシークレットだけである

   ```bash
   switchboard service install --relay
   switchboard service install
   ```

中継の状態は `switchboard service status --relay` で、ログは `journalctl -u switchboard-relay.service` で見る。webhookをやめるときは `switchboard relay hook remove <プロジェクト>` を実行し、設定から項目を消して `service install --relay` をやり直す。

GitLabは、4回続けて失敗したwebhookを一時的に止める。中継を長く止めると、webhookも止まりうる。止まっている間の更新も、照合で処理される。webhookを有効に戻すには、GitLabのプロジェクトの `Settings > Webhooks` でテストを送る。

署名トークンを使えないGitLabでは、`relay hook add --gitlab-token` で、`X-Gitlab-Token` を使って登録できる。このとき、設定の項目に `gitlab_token: true` が付く。`X-Gitlab-Token` は平文の共有シークレットなので、署名トークンが使えるならそちらを使う。

### 待ちと完了をTelegramに知らせる

issueが待ちになったときと完了したときに、エージェントがTelegramのbotから知らせる。待ちの通知には、プロジェクト、issue番号、タイトル、待ちの理由、質問の抜粋、issueのURLが載る。完了の通知には、作ったPRかMRのURLが載る。タイトルと質問の抜粋はTelegramのサーバーを通るので、載せたくないプロジェクトでは通知を使わない。

1. @BotFatherでbotを作り、そのbotに一言送ってから、chat idを調べる。既にあるbotを使ってもよい
2. botのトークンをSecret Serviceに置く。トークンは標準入力から読む

   ```bash
   secret-tool store --label='switchboard telegram' service switchboard/telegram username token
   ```

3. 設定に `notify` 節を足し、`switchboard service install` でエージェントを置き直す。botのトークンも、暗号化したクレデンシャルとしてサービスに渡る

   ```yaml
   notify:
     telegram:
       chat_id: "<chat id>"
       token_ref: switchboard/telegram
     web_url: http://192.168.0.82:8783   # 受信箱のWeb表示。あれば通知にリンクが付く
   ```

通知に失敗しても、エージェントの処理は止まらない。失敗は監査ログ（`~/.local/state/switchboard/audit.jsonl`）に `notify.failed` として残る。

### ブラウザで受信箱を見る

受信箱のWeb表示（`switchboard-web`）は、左に待ちの一覧、右に選んだissueの質問の全文を出し、そこから返答できる。デスクトップとスマホのどちらのブラウザでも使え、スマホでは一覧と1件を切り替えて出す。開いている間は、一覧と選んだissueを自動で更新する。issueの変化が画面に出るまでは、最大で35秒ほどかかる。仕組みは次の2つのサービスに分かれる。

- `switchboard inbox serve`：受信箱のトークンを持ち、Unixソケット（`/run/switchboard/inbox.sock`）で一覧、1件、返答のAPIを出す。一覧は30秒キャッシュする
- `switchboard-web serve`：そのAPIを呼び、LANのアドレスで画面を出す。フォージのトークンは持たない。開いている画面には、一覧をServer-Sent Events（`/api/events`）で送る。一覧は5秒ごとにAPIへ求め直し、変わったときだけ送る

Web表示は、端末で受信ポートを開く唯一の部品である（ADR-036）。LANの中だけに出す。今の版では、利用者を認証しない。そのため、LANの中の機器なら誰でも、自分の名前で返答できる。外のWebページからの返答は、`Origin` と `Host` の検査で拒む。

1. `~/.config/switchboard/web.yaml` に、待ち受けのアドレスを書く。この端末のLANのIPアドレスとポートを明示する。`0.0.0.0` は拒まれる。ホスト名は書けない。`Host` の検査を、この1つの値との比較にするためである。ブラウザでも、このIPアドレスで開く

   ```yaml
   listen: 192.168.0.82:8783
   ```

2. 2つのサービスを置く。受信箱のサービスには、各フォージの `inbox_token_ref` だけがクレデンシャルとして渡る。Web表示のサービスは、`PATH` にある `switchboard-web` を動かし、クレデンシャルを持たない

   ```bash
   switchboard service install --inbox
   switchboard service install --web
   ```

3. ブラウザで `http://192.168.0.82:8783` を開く。防火壁でポートを閉じているときは、LANからだけ開ける。firewalldなら、たとえば次のようにする

   ```bash
   sudo firewall-cmd --permanent --zone=public --add-rich-rule='rule family="ipv4" source address="192.168.0.0/24" port port="8783" protocol="tcp" accept'
   sudo firewall-cmd --reload
   ```

状態は `switchboard service status --inbox` と `--web` で見る。手で試すときは、`switchboard inbox serve` を動かしてから `switchboard-web serve` を動かす。このときソケットは `$XDG_RUNTIME_DIR/switchboard/inbox.sock` にでき、`web.yaml` の `socket` で場所を指定できる。

## 開発

```bash
make verify
```

`make verify` は、`web/` の検査（型の検査、lint、テスト、ビルド）も実行するので、Node.jsとpnpmが要る。画面の見た目の基準は [DESIGN.md](DESIGN.md)、使う人と目的は [PRODUCT.md](PRODUCT.md) にある。

実物の `claude` で、着手から質問、返答、完了までを通す確認もある。モデルを呼ぶため、`make verify` とCIでは動かない。

```bash
go test -tags realclaude -run RealClaude -v -timeout 30m ./internal/integration/
```

Secret Serviceが無い開発環境では、設定に `secrets: {allow_env: true}` を書く。するとSecret Serviceに項目が無いとき、環境変数からトークンを読む。変数の名前は、`token_ref` が `switchboard/github` なら `SWITCHBOARD_TOKEN_SWITCHBOARD_GITHUB` である。

## ライセンス

MIT。[LICENSE](LICENSE) を参照。
