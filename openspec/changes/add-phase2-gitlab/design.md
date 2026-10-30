# Design

## Context

動機と範囲はproposal.mdに、判断はgrilling.mdにある。設計の前提になる今のコードの形は、次のとおりである。

- `forge.Forge` は、GitHubの1実装（`internal/forge/github`）と偽物（`internal/forge/fake`）だけを持つ。`forge.IssueRef` は `Owner`・`Repo`・`Number` だけで、フォージの種別を持たない
- `agent.Agent`、`inbox.Inbox`、`trust.Judge` は、それぞれ `forge.Forge` を1つ、エージェントのユーザーIDを1つだけ持つ。`cmd/switchboard` が `github.New` で組み立てる
- 設定の `config.Project` は、`gh:owner/repo` を `Forge`・`Owner`・`Repo` に分けて持つ。`gl:` は読み込みの時点で拒否する
- `forge.Edit` には、編集者が分からないことを表す `Unknown` がすでにあり、信頼判定は `Unknown` を信頼済みでない編集として扱う
- 対象のGitLabは `gl.uskan.dev`（CE 19.5.1）である。CEなので、スコープ付きラベルは使えない

## Goals / Non-Goals

**Goals:**

- GitHubだけの設定では、Phase 1と同じ動きを保つ。Phase 1のテストは、型の変更への追従だけを行い、期待する結果は変えない
- GitLabのAPIへの依存を、`internal/forge/gitlab` に閉じ込める
- 偽のGitLabで、GitHubとGitLabを混ぜた設定の着手から完了までを、1本の結合テストで通す

**Non-Goals:**

- GitLabのwebhook（ADR-017）と中継：Phase 4で扱う
- 複数のGitLabインスタンス、プロジェクトごとの同時実行数
- 受信箱をGraphQLの1リクエストで取る形への変更（V14）
- GitLabのエピック、マージ列車、承認ルールなど、EEだけの機能

## Decisions

### 1. IssueRefにフォージの種別を持たせる

`forge.IssueRef` に `Forge`（`"gh"` か `"gl"`）を足す。`String()` は `gh:owner/repo#12` の形にし、受信箱の `<ref>` と同じ書き方にそろえる。GitLabのプロジェクトのパスは、最後の `/` で分け、前を `Owner`（`nas` や `nas/tools`）、後を `Repo` にする。

`Owner` と `Repo` を1つの `Path` にまとめる案もある。しかし、GitHubの呼び出し側の変更が大きくなる。`Owner` にサブグループを含める形なら、`FullName()` が `Owner + "/" + Repo` のままでどちらのフォージにも使える。

### 2. フォージはプロジェクトごとに選ぶ

`agent.Agent` の `Forge` を、表で置き換える。表のキーは、プロジェクトの設定の `ref`（`gl:nas/switchboard-sandbox`）である。表の値には、フォージのクライアントと、pushに使うトークンを入れる。そのプロジェクトのエージェントのID（`agent_identity` を解決したもの）も、同じ表に持たせる。プロジェクトに `token_ref` の上書きがあれば、そのプロジェクトだけ別のクライアントを作る。上書きが無いプロジェクトは、フォージの既定のクライアントを共有する。

`inbox.Inbox` は、フォージごとに `inbox_token_ref` のクライアントを1つずつ持つ。どちらも `cmd/switchboard` で組み立てる。

### 3. 信頼判定はフォージごとに解決する

`trust.Resolve` を、フォージごとに呼ぶ。渡すのは、そのフォージの `trusted_authors` と、そのフォージのプロジェクトが使う `agent_identity` の全部である。`trust.Judge` は、信頼済みのIDの集合をフォージごとに持つ。エージェントのIDはプロジェクトごとに持つ。

状態ディレクトリに記録するIDは、キーを `gh:uskanda` のようにフォージ付きにする。Phase 1の記録はフォージの区別が無いので、読み込むときに `gh:` 付きとみなす。これで、Phase 1から使っている端末の記録を、そのまま引き継げる。

### 4. GitLabのクライアント

標準ライブラリのHTTPクライアントで、REST（`/api/v4`）とGraphQL（`/api/graphql`）を呼ぶ。認証は `PRIVATE-TOKEN` ヘッダーで行う。GitLabのGo SDKは使わない。使うAPIは十数個で、依存を増やすほどの量ではないためである。

| 処理 | 使うAPI |
|---|---|
| プロジェクトの指定、公開範囲 | どのAPIでも、プロジェクトはURLエンコードしたパスで指定する。GitLabは、IDの代わりにパスを受け付ける。`doctor` は `GET /projects/:path` でIDを引き、パスが変わっていないかを確かめる。`visibility` が `private` のときだけ非公開とする |
| 更新の検知 | `GET /projects/:id/issues?updated_after=…&order_by=updated_at`。初回は `updated_after` を付けない。ノートではissueの `updated_at` が変わらない（V22）ため、待ちのissueは別に一覧する（11節） |
| issueとコメント、編集者 | GraphQLのissueの `notes` で、ノートの作成者、`lastEditedBy`、`system` を取る。本文の作成者と本文はRESTの `GET /projects/:id/issues/:iid` で取る |
| 本文の最後の編集者 | 本文の変更を示すシステムノート（本文に `description` を含むもの）の、最後の作成者（V21）。ノートが取得の範囲を超えて、確かめきれないときは `Unknown` とする |
| ラベルを付けた人 | `GET /projects/:id/issues/:iid/resource_label_events` の、そのラベルの最後の `add` の `user` |
| ラベルの付け替え | `PUT /projects/:id/issues/:iid` に `labels` を全置換で渡す。GitHubと同じく、直後に取り直して確かめる |
| コメントの投稿 | `POST /projects/:id/issues/:iid/notes` |
| ラベルの作成 | `GET /projects/:id/labels` で有無を見て、無ければ `POST /projects/:id/labels` |
| draftのMR | `POST /projects/:id/merge_requests`。タイトルの先頭に `Draft: ` を付ける |
| ユーザーIDの解決 | `GET /users?username=…` |
| 受信箱 | 設定済みのプロジェクトごとに、`GET /projects/:id/issues?labels=…&state=opened` を並列に呼ぶ |

システムノートは、信頼判定ではコメントとして扱わない。プロンプトにも入れない。ノートの `internal`（内部メモ）は、ほかのノートと同じ規則で判定する。

### 5. GitLabへのpush

`<workspace.Space.Push>` は、httpsのリモートに `GIT_ASKPASS` でトークンを渡している。GitLabでは、ユーザー名を `oauth2` にする。GitHubは今の `x-access-token` のままにする。sshのリモートは、Phase 1と同じく、トークンを渡さずに `git push` する。

### 6. worktreeの片付け

ポーリングで閉じられたissueを見つけたら、そのissueのworktreeがあり、実行が動いていないときに片付ける。手順は次のとおりである。

1. worktreeで `git status --porcelain` を実行する。出力が空でないときは、消さずにログへ書く
2. 空なら `git worktree remove <worktree>` と `git branch -D switchboard/issue-<n>` を実行する

実行が動いているissueは、取り消しで実行が止まった後の、次のポーリングで片付ける。リモートのブランチには触れない。

`git branch -D` は、pushしていないコミットがあっても消す。issueが閉じられた時点で作業は終わったとみなす。また、完了した実行のブランチはpush済みである。

### 7. 受信箱のマージ

フォージごとに一覧を取り、成功した側だけを、待ち始めた時刻で並べ直す。待ち始めた時刻は、Phase 1と同じくマーカーの `at` から取る。失敗したフォージは、理由を標準エラーに書き、コマンドを0以外で終える。`--forge` は、取得の前に対象のフォージを絞る。行番号の記録には `gl:` の形の参照を保存し、`show` と `reply` が正しいフォージを選べるようにする。

### 8. 設定

```yaml
forges:
  github: {token_ref: …, inbox_token_ref: …, agent_identity: …, search_scope: …}
  gitlab:
    base_url: https://gl.uskan.dev   # 既定は https://gitlab.com
    token_ref: switchboard/gitlab
    inbox_token_ref: switchboard/gitlab-inbox
    agent_identity: uskanda
trusted_authors:
  github: [uskanda]
  gitlab: [uskanda]
projects:
  - ref: gl:nas/switchboard-sandbox
    token_ref: switchboard/gitlab-sandbox-bot     # 任意。エージェントのトークンだけを上書きする
    agent_identity: project_42_bot_…              # 任意。上書きするときは token_ref と組で書く
```

`token_ref` と `agent_identity` は、片方だけの上書きを設定の誤りにする。片方だけだと、トークンの持ち主とエージェントの身元がずれ、エージェント自身の投稿を見分けられなくなるためである。GitLabの設定では `search_scope` を受け付けない。受信箱は、設定済みのプロジェクトだけを見るためである。

### 9. doctor

`doctor` は、GitLabのプロジェクトについても、トークン、ユーザーID、ローカルのリポジトリ、公開範囲を調べる。加えて、`GET /version` でGitLabの版を表示する。プロジェクトに上書きがあれば、そのトークンとbotのユーザーIDも調べる。

### 10. ADR

DECISIONS.mdに、次の2つを足す。

- ADR-030（GitLabの扱い）：対象のインスタンスは1つで、既定は `https://gitlab.com`。プロジェクトはパスで書き、起動時にIDを引く。`internal` は公開として数える
- ADR-031（エージェントの身元の上書き）：エージェントのトークンと `agent_identity` は、組にしてプロジェクトの設定で上書きできる。受信箱のトークンは、フォージに1つとする

ADR-017の論点は、「Phase 4の中継で決める」に書き直す。

### 11. GitLabでの返答の検知

GitLabでは、ノートを足してもissueの `updated_at` が変わらない（V22）。そのため、更新されたissueの一覧だけでは、待ちのissueへの返答を見落とす。

GitLabのプロジェクトでは、ポーリングのたびに `GET /projects/:id/issues?labels=<待ちのラベル>&state=opened` も呼ぶ。返る `user_notes_count` が前回の値と違うissueだけを、更新されたissueと同じく評価する。前回の値は、エージェントのメモリに持つ。起動した直後は、待ちのissueをすべて評価する。

ノートの一覧を毎回取る案もある。しかし、待ちのissueの数だけリクエストが増える。`user_notes_count` なら、1プロジェクトにつき1リクエストで済む。

## Risks / Trade-offs

- ［本文の編集者を、システムノートの文言で見分ける］→GitLabの版で文言が変わると、本文の編集を見落とし、編集されていない本文として扱う。対策として、本文に `description` を含むシステムノートを、すべて本文の変更として扱う。文言は偽のGitLabのテストに固定する。版を上げたら、V21を確かめ直す。非公開のプロジェクトで本文を編集できるのはメンバーだけなので、見落としの影響はメンバーの範囲に留まる
- ［ラベルの全置換は、後から書いた側が残る］→GitHubと同じく、付け替えの後に取り直し、状態ラベルが2つ以上なら正規化する
- ［GitLabのPATは、自分の全プロジェクトを操作できる］→自分の環境ではPATを使うと決めた（grilling第2ラウンドQ6）。公開した製品では、プロジェクトの上書きでProject Access Tokenを使えるようにし、READMEで勧める
- ［`git branch -D` で、pushしていないコミットを消しうる］→消すのはissueが閉じられた後だけで、コミットしていない変更があれば消さない
- ［受信箱は、GitLabのプロジェクトの数だけAPIを呼ぶ］→個人の規模では数個なので、並列に呼べば十分に速い。増えたらV14と合わせて見直す

## Migration Plan

- Phase 1の設定は、そのまま読み込める。GitLabを使うときだけ、`forges` と `trusted_authors` に `gitlab` を書き足す
- 状態ディレクトリのユーザーIDの記録は、フォージの区別が無いものを `gh:` として読み込み、次に書くときにフォージ付きのキーで書き直す
- 戻すときは、Phase 1の版のバイナリに入れ替える。フォージ付きのキーの記録は、Phase 1の版では別のログイン名として読まれる。そのため、GitHubの記録は書き直しになるが、起動は止まらない

