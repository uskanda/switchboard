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
| エージェント（ラベル、コメント、push、PR、CIの見張り） | `token_ref` | Contents、Issues、Pull requestsの読み書き。Actions、Commit statusesの読み取り |
| 受信箱（一覧、返答、WUIからのマージ） | `inbox_token_ref` | Issues、Pull requests、Contentsの読み書き |

```bash
secret-tool store --label='switchboard github' service switchboard/github username token
```

```bash
secret-tool store --label='switchboard github inbox' service switchboard/github-inbox username token
```

どちらのコマンドも、トークンを標準入力から読む。トークンは `claude` には渡さない。

エージェントのトークンにActionsとCommit statusesの読み取りが無いと、エージェントはPRのCIを見張らない。ほかの動きは変わらない。`switchboard doctor` が警告を出す。

受信箱のトークンのPull requestsとContentsの書き込みは、受信箱のWeb表示から完了のPRをマージするために使う（ADR-043）。マージは人の身元で行い、フォージにも人のマージとして残る。今のトークンに足すときは、GitHubのトークンの設定で、Pull requestsとContentsを「Read and write」にして保存する。トークンの値は変わらないので、登録し直さなくてよい。足さないうちは、マージだけが「権限がありません」で断られる。

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

対象のリポジトリに、状態ラベル（`agent:ready`、`agent:plan`、`agent:go`、`agent:running`、`agent:waiting`、`agent:done`）を作る。既にあるラベルは作り直さない。

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

`agent:done` のissueも、closeするまでは作業を続けられる。PRに問題があるときやコンフリクトしたときは、issueに返信すると、次のポーリングで再び実行する。実行は同じブランチに積み、開いたPRがあれば新しく作らずに使い続ける。そのときの完了のコメントは「PRを更新しました」になる。baseとの衝突は `git merge` で解消させ、rebaseはさせない。pushはforceなしのままである。

実行の間、エージェントは進捗のコメントを1つ投稿し、1分ごとに書き換えて1行ずつ足す。1行には、時刻、経過時間、使ったツールの名前と回数、`claude -p` が最後に書いたテキストの1行目（120文字まで）を書く。前の行と同じ内容なら足さない。ツールの入力と出力（コマンドの中身、ツールに渡したファイルのパス）は書かない。ただし、`claude -p` のテキストにファイルの名前があれば、目的の1行に出る。書き換えのたびに、秘密の検査と `comments.check_command` を通し、秘密を含むように見える行は伏せる。誰かが進捗のコメントを消したら、次の行で投稿し直す。1分より早く終わる実行には、進捗のコメントは付かない。進捗のコメントは、結果のコメントを投稿したあとと、実行を取り消したあとに消える。エージェントが落ちて残ったものは、再起動のあとの回復で消える。

フォージへの要求が一時的に失敗しても、issueは止まったままにならない。結果の書き戻しに失敗した実行は、ポーリングのたびに書き戻し直す。回数に上限は無い。書き戻し直す前にissueを読み直す。issueがcloseされているか、人がラベルを変えていたら、ラベルは変えない。その場合、未投稿の結果のコメントだけを投稿し、通知は送らない。評価の途中で失敗したissueも、次のポーリングで評価し直す。やり直すものがある間は、中継につながっていても `poll_interval` ごとにポーリングする。

別の端末から、エージェントの状態を見る。

```bash
switchboard agent status
```

書き戻しを待つ実行は、`runs:` の欄に `write-back pending since <時刻>` と出て、実行中の行とは区別される。

全プロジェクトの待ちのissueを、GitHubとGitLabをまとめて、待ち始めた順に一覧する。その下の「Running」の欄に、`agent:running` のissueを、端末、経過時間、進捗のコメントの最後の行とともに出す。進捗のコメントがまだ無ければ `?` と出る。一番下の「Done (waiting for close)」の欄に、openのままの `agent:done` のissueを、端末、完了からの時間、PR/MRのURLとともに、完了の古い順に出す。closeすると一覧から消える。行番号は、待ちの行から完了の行へ続けて付く。`--json` を付けると、JSONの配列が出る。各要素の `state` は、`waiting`、`done`、`running`、`queued` のどれかになる。`--forge github` か `--forge gitlab` で、片方のフォージに絞れる。片方のフォージに届かないときは、届いた側の一覧を出したうえで、0以外の終了コードで終わる。

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

### 計画を先に立てる

実装の前に、エージェントにissueのコメントで質問させ、計画を承認してから実装させられる。始めるときに付けるラベルは、いつも1つである。

- `agent:ready`：プロジェクトの `plan_first` に従う。`plan_first: true` なら計画から始め、無いか `false` ならすぐ実装する
- `agent:plan`：計画から始める
- `agent:go`：すぐ実装する

計画の段階の実行は、ファイルを1つも変えない。使えるのは、読み取りのツールと `git status`、`git diff`、`git log` だけである。終わったあとにworktreeの変更があれば捨てる。pushもPRの作成もしない。実行は、決まっていない判断を推奨の答えとともに1回のコメントでまとめて聞く。決めることが無くなったら、計画をコメントする。どちらも `agent:waiting` になり、待ちの理由は `plan-questions` と `plan` である。

返答は、承認の語（既定は `/ok`）で始める。

- 質問に `/ok` と返すと、推奨どおりの答えとして次のラウンドに進む。`/ok q2はB` のように続けると、その問いだけを変えられる。質問の番号は、ラウンドをまたいで続く
- 計画に `/ok` と返すと承認になり、次の実行で実装に入る。`/ok` 以外の返答は、計画の直しの依頼として、計画を出し直す
- 承認の語は、信頼済みの人のコメントの最初の行の最初の語だけを見る。`/okay` や、信頼済みでない人に編集されたコメントは承認にならない

承認の語は設定で変えられる。

```yaml
plan:
  approve_word: /ok
projects:
  - ref: gh:YOUR_GITHUB_USER/switchboard
    plan_first: true
```

計画の段階の実行には、利用者の追加の指示を渡せる。全プロジェクトに1つと、プロジェクトごとに1つで、両方あれば全体、プロジェクトの順につなげる。実装の実行には渡らない（実装の実行には、別の `run.md` を渡せる）。指示は、設定ファイルと同じディレクトリの `instructions/` の下にある、ただのファイルである。全体は `instructions/plan.md` である。GitHubのプロジェクトの指示は `instructions/gh/<owner>/<repo>/plan.md` に置く。GitLabなら `instructions/gl/<group>/<project>/plan.md` である。

```bash
switchboard instructions edit                 # 全プロジェクトの指示
switchboard instructions edit gh:o/switchboard  # 1つのプロジェクトの指示
```

受信箱のWeb表示の「設定」の画面からも編集できる。保存は32 KiBまでで、秘密を含むように見えるものは拒まれる。保存のたびに、監査ログに内容のSHA-256が残る。ファイルは端末ごとにある。Web表示が変えるのは、Web表示の端末のファイルだけである。エージェントが別の端末で動くときは、その端末で編集するか、設定ファイルと一緒に同期する。

### 実装の実行に足す許可と指示

ファイルを書く実行（直接の実装、承認のあとの実装、CIとpushの失敗を直す実行、baseとの衝突を直す実行）には、許可するツールと利用者の追加の指示を足せる。手作業で使っているスキルやCLIを、自動の実行でも使わせるためのものである。計画の実行には、どちらも足さない。

許可は、設定ファイルにClaude Codeの権限の規則の形で書く。`allowed-tools` を持つスキル（多くのハーネスのスキル）は、`Skill` を許可しないと呼べない。全体の `agent.allow_tools_extra` と、プロジェクトごとの `allow_tools_extra` をつなげて使う。

```yaml
agent:
  allow_tools_extra: [Skill]
projects:
  - ref: gh:YOUR_GITHUB_USER/switchboard
    allow_tools_extra: ["Bash(openspec *)"]
```

設定を読むときに、空の項目、制限の無い `Bash` と `Bash(*)`、どの実行でも拒否する操作（`git push`、`git rebase`、`gh`、`WebFetch`）を名指しする項目は誤りになる。`Bash(git *)` のような広い項目は通るが、拒否する操作は拒否されたままである。プロジェクトの `allow_tools_extra` は再起動せずに反映され、`agent.allow_tools_extra` は再起動で反映される。許可は設定ファイルにだけ書け、受信箱のWeb表示からは変えられない。

指示は、計画の指示と同じディレクトリに `run.md` の名前で置く。全体は `instructions/run.md` である。GitHubのプロジェクトは `instructions/gh/<owner>/<repo>/run.md` に置く。GitLabなら `instructions/gl/<group>/<project>/run.md` である。保存の検査と監査ログは計画の指示と同じで、Web表示の「設定」の画面からも編集できる。

実行を始めるたびに、監査ログに `run.started` が残る。中身は、実行のID、実行の種類、足した許可の一覧、使った指示のファイルごとのSHA-256である。計画の実行では、足した許可は空で、ハッシュは `plan.md` のものになる。同じ値を持つ `instructions.saved` の行から、どの保存の版で動いたかが分かる。指示の内容そのものは残らない。

```bash
switchboard instructions edit --run                   # 全プロジェクトの実装の指示
switchboard instructions edit --run gh:o/switchboard  # 1つのプロジェクトの実装の指示
```

OpenSpecを使うプロジェクトでの書き方の例は、[docs/examples/openspec.md](docs/examples/openspec.md) にある。

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

置いてあるサービス（エージェント、中継、受信箱、Web表示）を、まとめて動かし直せる。置いていないサービスは飛ばす。sudoのパスワードを端末で入力する。

```bash
make restart
```

作り直した版で動かし直すときは、`make deploy` を使う。2つのバイナリを作って `~/.local/bin` に置いてから、`make restart` と同じことをする。置き場所は `PREFIX` で変えられる。チェックアウトがmain以外のブランチにあるときは警告を出す。端末から実行していれば、続けるかを尋ね、`y` 以外の答えで止まる。

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

中継につながっている間、エージェントは、`relay.hooks` にwebhookを登録したプロジェクトを `reconcile_interval`（既定10分）ごとに照合する。webhookの無いプロジェクト（たとえばGitHubのプロジェクト）には中継からイベントが届かないので、中継につながっていても `poll_interval` ごとにポーリングする。中継が止まったら、すぐに全プロジェクトが `poll_interval` のポーリングに戻り、中継が戻ったらつなぎ直す。

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

受信箱のWeb表示（`switchboard-web`）は、待ちと実行中の一覧と、1件のissueのスレッドを出し、そこから返答できる。

- 画面の上のヘッダに、待ちと実行中の件数、自動更新の状態、設定へのリンクが出る
- 幅1024px以上では、左に一覧、右に選んだissueが並ぶ。それより狭い幅（スマホ）では、一覧と1件を別の画面で行き来する
- 1件の表示には、issueの本文とコメントが古い順に出る。本文とコメントはMarkdownとして描く。生のHTMLは文字のまま出し、HTMLのコメント（`<!-- ... -->`）は出さない。画像は読み込まずにリンクとして出す。エージェントの投稿には「エージェント」の印が付く。着手のコメントは1行で、進捗のコメントは最新の1件だけが出る
- 実行中の欄には、端末、経過時間、最新の進捗の1行が出る。実行中のissueには返答できない
- 順番待ちの欄は実行中の欄の下にあり、当たった上限と、順番待ちになってからの時間が出る。返答の待ちから順番待ちになったissueには、今までどおり返答できる
- 「完了（close待ち）」の欄は順番待ちの欄の下にあり、端末、完了からの時間、PR/MRへのリンクが出る。ヘッダの完了の件数は、1件以上のときだけ出る。完了のissueにも返答でき、返答すると同じPR/MRで作業を再開する。issueをcloseすると欄から消える
- 完了の行と1件の表示には「マージ」のボタンが出る。確認のダイアログで「マージする」を押すと、受信箱のトークンでPR/MRのdraftを解除してマージする。マージするのは、エージェントが完了を知らせたときのコミットだけで、その後にPR/MRへコミットが足されていれば断る。CIの結果やコンフリクトでフォージが断ったときは、その理由がトーストに出る。issueは、PR/MRの本文の `Closes #N` でcloseされ、一覧から消える。baseが既定のブランチでないときはissueが閉じないので、手でcloseする（ADR-043）
- 実行の失敗（理由が `error`）の待ちには「再実行する」のボタンが出る。返答の欄が空のときに押すと再実行を頼む決まった文を返答し、エージェントが同じ作業をやり直す
- 画面を開いている間は、一覧と選んだissueが自動で更新される。返答の書きかけは、更新されても消えない
- 参照の前に、プロジェクトのアイコンが出る。既定はフォージのアバターで、GitLabはプロジェクトのアバター（無ければグループのアバター）、GitHubはオーナーのアバターを使う。受信箱のサービスが受信箱のトークンで取り、`~/.local/state/switchboard/project-icons/` に24時間保存する。取れないときはプロジェクト名の頭文字が出る。設定のプロジェクトに `icon` を書くと、そちらが優先される（ADR-048）

  ```yaml
  projects:
    - ref: gh:YOUR_GITHUB_USER/switchboard-sandbox
      icon: "<絵文字1文字>"                  # 絵文字は文字のまま出る
    - ref: gl:YOUR_GROUP/tools
      icon: ~/icons/tools.png               # 端末の画像ファイル（/か~/で始める）
    - ref: gh:YOUR_GITHUB_USER/notes
      icon: https://example.com/notes.png   # 画像のURL（https://だけ。トークンは付けない）
  ```

  画像はPNG、JPEG、GIF、WebPの256KiBまでで、SVGは使えない
- 一覧の上の「新規issue」から、設定にあるどのプロジェクトにもissueを作れる。開始ラベルは、設定の `labels.ready`、`labels.plan`、`labels.go` から1つを選ぶ（初期値はready）。issueは受信箱のトークンで作るので、エージェントがそのまま着手する（ADR-040）

仕組みは次の2つのサービスに分かれる。

- `switchboard inbox serve`：受信箱のトークンを持つ。Unixソケット（`/run/switchboard/inbox.sock`）で、一覧、1件、返答のAPIと、一覧の変化を知らせるSSEを出す。一覧は60秒キャッシュする。SSEでつないでいる画面がある間だけ、フォージから一覧を取り直し、変わったときに知らせる。プッシュ通知の設定（`inbox.notify.fcm`）があるときは、画面が無くても取り直す。取り直す間隔は `inbox.refresh_interval` で、既定は30秒、下限は10秒である
- `switchboard-web serve`：そのAPIを呼び、LANのアドレスで画面を出す。SSEはブラウザへ中継する。フォージのトークンは持たない

画面が見えている間は、フォージへの問い合わせが続く。待ちが5件で間隔が30秒なら、GitHubのREST APIを1時間に約600回、検索を約120回使う。タブが裏に回ると、画面はSSEを閉じる。見えている画面が無くなれば、問い合わせも止まる。減らしたいときは、設定ファイルで間隔を延ばす。

```yaml
inbox:
  refresh_interval: 60s
```

Web表示は、端末で受信ポートを開く唯一の部品である（ADR-036）。LANの中だけに出す。今の版では、利用者を認証しない。そのため、LANの中の機器なら誰でも、自分の名前で返答でき、issueを作って実行を起こせる（ADR-040）。外のWebページからの返答とissueの作成は、`Origin` と `Host` の検査で拒む。

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

#### 中継のイベントで一覧をすぐ取り直す

中継（[中継で返答にすぐ着手する](#中継で返答にすぐ着手する)）を置いた端末では、受信箱のサービスも中継につなげられる。webhookのイベントが届くと、`inbox.refresh_interval` を待たずに一覧を取り直す。続けて届いたイベントは、最初のイベントから2秒の間をまとめて1回にする。前回の取り直しから10秒たつまでは取り直さない。webhookの無いプロジェクト（GitHubのプロジェクトなど）の変化は、これまでどおり `inbox.refresh_interval` ごとの問い合わせで画面に出る。中継につないでいても、この間隔は延ばさない。

受信箱のサービスは、サービスが動いている間ずっと中継につなぐ。画面を開いていないときにイベントが届いたら、一覧のキャッシュを捨て、次に画面を開いたときに取り直す。

1. 受信箱のサービス用のトークンを作る。エージェントとは別のトークンになり、`switchboard/relay-<端末名>-inbox` に置かれる。画面には、`relay.hosts` に足す項目（名前は `<端末名>-inbox`）と、`inbox.relay` に足す項目が出る

   ```bash
   switchboard relay token add --inbox <端末名>
   ```

2. 表示された項目を、設定ファイルの `relay.hosts` と `inbox:` 節に貼り付ける

   ```yaml
   relay:
     hosts:
       - name: <端末名>-inbox
         token_hash: sha256:<16進>
   inbox:
     relay:
       url: http://127.0.0.1:8782/events
       token_ref: switchboard/relay-<端末名>-inbox
   ```

3. 中継と受信箱のサービスを置き直す。受信箱のサービスには、このトークンも暗号化したクレデンシャルとして渡る

   ```bash
   switchboard service install --relay
   switchboard service install --inbox
   ```

ヘッダの自動更新の表示は、中継の状態で次のように変わる。中継につながったときと切れたときは、受信箱のサービスのログ（`journalctl -u switchboard-inbox.service`）にも出る。

| 表示 | 意味 |
|---|---|
| 自動更新中（即時） | 中継につながっている。webhookのあるプロジェクトの変化は数秒で出る |
| 自動更新中（30秒ごと） | 中継が切れている。変化は `inbox.refresh_interval` ごとに出る（数字はその秒数） |
| 自動更新中 | `inbox.relay` を設定していない |

#### 画面からプロジェクトを足す

設定の画面の「プロジェクト」の節に、この端末のプロジェクトが並ぶ。それぞれ、クローンの有無と、足りない状態ラベルが出る。`config.yaml` に `inbox.project_root` を書くと、この節からプロジェクトを足せるようになる。

```yaml
inbox:
  project_root: ~/repos               # 足したプロジェクトのクローンを置く親のディレクトリ
```

入力するのは、参照（`gh:owner/repo` または `gl:group/project`）と、計画から始めるかどうか（初期値はオン）だけである。受信箱のサービスは、受信箱のトークンでリポジトリを読めることと既定のブランチを確かめ、`config.yaml` の `projects` の最後に1項目を足す。`path` は `<project_root>/<リポジトリ名>`、`host` はこの端末、`base_branch` は既定のブランチになる。コメントとほかの項目はそのまま残り、元のファイルは `config.yaml.bak` に残る。`verify_command` などのほかの項目は、`config.yaml` を手で編集して書く。

エージェントと受信箱のサービスは、10秒ごとに `config.yaml` を確かめ、`projects`、`agent.max_parallel`、`agent.prefer_plan` だけを読み直す。再起動は要らない。新しいプロジェクトは、エージェントが自分のトークンでcloneし、状態ラベルを作ってから扱い始める。失敗したら、次の照合でやり直す。cloneできないときは、エージェントのトークン（fine-grainedのPATなら、その対象のリポジトリ）を確かめる。それ以外の変更は、今までどおり再起動で反映する。

同時に動かす実行の数は、端末全体の `agent.max_parallel`（既定は1）と、プロジェクトごとの `max_parallel` で制限する。プロジェクトに書かなければ、全体の上限だけで制限する。実際の上限は2つの値の小さい方になる。上限に達したプロジェクトのissueは順番待ちになり、ほかのプロジェクトのissueは先に実行される。上限を下げても、動いている実行は止まらない。

スロットが空くと、順番待ちに入った順にissueを始める。`agent.prefer_plan: true`（既定はfalse）にすると、順番待ちのうち計画の実行（計画を先に立てるモードで、質問か計画を出す実行）を先に始め、実装の実行は後に回す。並べ替えるのは順番待ちから取り出すときだけで、スロットが空いていれば実装もすぐに始まる。エージェントを再起動すると、順番待ちに入った時刻は消え、再評価で入り直した順になる。

### Androidのアプリで見て、通知を受ける

`app/` のAndroidのアプリは、受信箱のWeb表示をWebViewで開く。画面はWeb表示と同じで、Web表示を新しくすればアプリにも反映される。アプリが足すのは次のものである（ADR-047）。

- ホーム画面から全画面で開ける。Web表示の外のリンク（フォージのページなど）は、既定のブラウザで開く
- 待ちと完了のプッシュ通知が届く。通知を押すと、そのissueが開く
- 計画の承認待ちの通知には「承認」のボタンが付き、アプリを開かずに承認の語を返答する。通知を受けた後に計画が出し直されていれば、承認せずにそのことを知らせる
- 完了の通知には「マージ」のボタンが付き、アプリでマージの確認のダイアログが開く

通知は受信箱のサービスがFCMで送る。Telegramの通知はエージェントが送るので、両方を設定すれば両方に届く。通知に載るのは、issueの参照、待ちの理由、タイトルだけで、質問の本文は載らない。これらはGoogleのサーバーを通る。家のWi-Fiの外では、通知は届いても、アプリからWeb表示は開けない。

1. Firebaseのプロジェクトを作り、Androidのアプリを、パッケージ名 `io.github.uskanda.switchboard` で登録する。`google-services.json` を、リポジトリの外に保存する
2. Firebaseのコンソールの「サービスアカウント」で秘密鍵（JSON）を作り、Secret Serviceに置く。鍵は標準入力から読む

   ```bash
   secret-tool store --label='switchboard fcm' service switchboard/fcm username token < firebase-adminsdk.json
   ```

3. 設定に `inbox.notify.fcm` を足し、`switchboard service install --inbox` で受信箱のサービスを置き直す。鍵も、暗号化したクレデンシャルとしてサービスに渡る。設定があると、受信箱のサービスは画面が無くても `inbox.refresh_interval` ごとに一覧を取り直す

   ```yaml
   inbox:
     notify:
       fcm:
         project_id: <Firebaseのプロジェクトのid>
         credentials_ref: switchboard/fcm
   ```

4. 署名の鍵を作る。鍵とパスワードは、リポジトリの外に置く

   ```bash
   keytool -genkeypair -v -keystore ~/keys/switchboard-app.keystore -alias switchboard -keyalg RSA -keysize 2048 -validity 10000
   ```

5. APKを作る。JDK 17と、Android SDKのplatform 37（パッケージ名は `platforms;android-37.0`）とbuild-tools 37.0.0、NDK 27.1が要る。足りなければ `sdkmanager "platforms;android-37.0" "build-tools;37.0.0" "ndk;27.1.12297006"` で入れる。`ANDROID_HOME` と `JAVA_HOME` を設定し、鍵と `google-services.json` の場所を環境変数で渡す。足りない変数があれば、Gradleの前で止まる。できたAPKは `bin/switchboard.apk` に置かれる

   ```bash
   export SWITCHBOARD_APP_KEYSTORE=~/keys/switchboard-app.keystore SWITCHBOARD_APP_KEY_ALIAS=switchboard
   export SWITCHBOARD_APP_KEYSTORE_PASSWORD=... SWITCHBOARD_APP_KEY_PASSWORD=...
   export SWITCHBOARD_APP_GOOGLE_SERVICES=~/keys/google-services.json
   make app-apk
   ```

6. APKをスマホに入れて開き、Web表示のURLを入力する。`web.yaml` の `listen` と同じIPアドレスとポートを、`http://` を付けて入力する（例：`http://192.168.0.82:8783`）。ホスト名で入力すると、`Host` の検査で返答と承認が拒まれる。URLは、画面の上の「接続先」から変えられる

アプリは、受信箱のWeb表示に、通知のトークンの登録と承認をPOSTで送る。どちらも、返答と同じ `Origin` と `Host` の検査を通る。アプリは、入力したURLを `Origin` として付ける。

### pushとCIの失敗

エージェントは、PRかMRを作って `agent:done` にした後も、自分がpushしたコミットのCIを `poll_interval` ごとに見張る。中継につながっていても同じ間隔で見る。CIが全部終わって失敗が1つも無ければ、何も書かずに見張りをやめる。結論が `failure` か `timed_out` のものがあれば、issueを `agent:running` に戻し、「CIが失敗したので直します（1/2）」のコメントを付けて直す実行を始める。直す実行には、失敗したjob（最大3つ）のログの末尾200行を、非信頼のデータとして渡す。直した結果は同じPRにpushし、もう一度見張る。

直す回数は、1つのissueで `agent.max_fix_attempts`（既定は2）までである。上限まで直してもCIが失敗したら、issueは理由 `ci` の待ちになる。コメントには、失敗したチェック、各回の要約、ログの末尾が載る。信頼済みの人が返信するか、始めるラベルを付け直すと、回数は0から数え直す。0にすると、自動では直さない。

次のときは、何も書かずに見張りをやめる。PRが閉じられたかマージされたとき、人が同じブランチに別のコミットをpushしたとき、pushから10分たってもCIが1つも現れないとき、pushから6時間たってもCIが終わらないとき、トークンがCIを読めないときである。

pushが失敗したときは、gitの出力で扱いを分ける。ネットワークなどの一時的な失敗は、同じ実行の中で3回まで試す。リモートのissueのブランチに人のコミットがある（non-fast-forward）ときは、そのブランチを取り込む直す実行を始める。直す実行には `git merge` だけを許し、強制pushはしない。トークンの権限の不足と、ブランチの保護やpre-receive hookなどリモートの規則による拒否は、理由 `push` の待ちにする。コメントには、gitの出力と、人がすべきことが載る。

```yaml
agent:
  max_fix_attempts: 2   # 0で自動の直しを切る
```

順番待ちになったissueには、エージェントが「順番待ちにしました」というコメントを1つ付ける。コメントには、当たった上限が端末かプロジェクトかと、その値だけを書く。ラベルは変えない。順番が来て着手するとき、または始めるラベルが外れたときやcloseされたときに、エージェントはこのコメントを消す。エージェントを再起動したときは、前のプロセスが付けたものを消し、順番待ちのままなら付け直す。ログには、順番待ちに入ったことと、順番待ちから始めたことが1行ずつ出る。

```yaml
agent:
  max_parallel: 3
  prefer_plan: true   # 順番待ちから計画の実行を先に取り出す
projects:
  - ref: gh:YOUR_GITHUB_USER/switchboard
    max_parallel: 1
```

次の場合は追加を拒む。

- 同じプロジェクトが既に設定にある
- この端末のプロジェクトが、まだそのフォージを使っていない。`config.yaml` に手で足して、`switchboard service install` を実行し直す。エージェントにそのフォージのトークンを渡すためである
- 受信箱のトークンでリポジトリを読めない
- 別のプロジェクトが、同じ場所にクローンされている

中継を使う端末でGitLabのプロジェクトを足すと、画面が `switchboard relay hook add <ref>` の実行を案内する。webhookを登録するまでは、そのプロジェクトはポーリングで扱われる。GitHubのwebhookは中継に届かないので、GitHubのプロジェクトには案内を出さない。

状態は `switchboard service status --inbox` と `--web` で見る。手で試すときは、`switchboard inbox serve` を動かしてから `switchboard-web serve` を動かす。このときソケットは `$XDG_RUNTIME_DIR/switchboard/inbox.sock` にでき、`web.yaml` の `socket` で場所を指定できる。

## 開発

```bash
make verify
```

`make verify` は、`web/` の検査（型の検査、lint、テスト、ビルド）と、`app/` の検査（型の検査、lint、テスト）も実行するので、Node.jsとpnpmが要る。AndroidのAPKを作る `make app-apk` だけは、JDKとAndroid SDKも要る。画面の見た目の基準は [DESIGN.md](DESIGN.md)、使う人と目的は [PRODUCT.md](PRODUCT.md) にある。

実物の `claude` で、着手から質問、返答、完了までを通す確認もある。モデルを呼ぶため、`make verify` とCIでは動かない。

```bash
go test -tags realclaude -run RealClaude -v -timeout 30m ./internal/integration/
```

Secret Serviceが無い開発環境では、設定に `secrets: {allow_env: true}` を書く。するとSecret Serviceに項目が無いとき、環境変数からトークンを読む。変数の名前は、`token_ref` が `switchboard/github` なら `SWITCHBOARD_TOKEN_SWITCHBOARD_GITHUB` である。

## ライセンス

MIT。[LICENSE](LICENSE) を参照。
