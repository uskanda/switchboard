# Tasks

## 1. 既定のgitとfetch済みの案内

- [x] 1.1 `internal/agent` に失敗するテストを書く。実装の実行の設定の許可に、足すサブコマンド（`git ls-tree`、`git mv`、`git stash` など）の規則が入ることを確かめる。`git fetch`、`git checkout`、`git config` の規則が入らないことも確かめる。計画の実行の設定には、読むだけのものだけが入り、`git mv` が入らないことも確かめる
- [x] 1.2 `gitRead` と `gitWorktree` の一覧を足し、`baseAllow` と `planBaseAllow` で使う（design.mdのD1）。1.1のテストと、既存の `allow_test.go`、`tools_test.go` が通ることを確かめる
- [x] 1.3 `internal/prompt` に失敗するテストを書く。計画の実行、実装の実行、直す実行のsystem promptに、`origin/<base>` がfetch済みであることと `git fetch` が許されないことが入ることを確かめる。system promptは英語だけなので、言語の設定によらず同じ文が入る。D2のとおり組み込みの節に足して通す

## 2. 許可のファイルと検査

- [x] 2.1 `internal/config` の `checkAllowTools` から、1つの規則を調べる関数を切り出して公開する。既存の `config` のテストが通ることを確かめる
- [x] 2.2 `internal/permissions` に失敗するテストを書く。全体、GitHubとGitLab（入れ子のグループを含む）のプロジェクト、issueのファイルの置き場所と、0600と0700の権限を確かめる。読み込みでは空行、`#` の行、重複が除かれることを確かめる。追加と削除のあとも、手で書いた行とコメントが残ることを確かめる
- [x] 2.3 `Store` の `Read`、`Add`、`Remove` を実装する（D3）。2.2のテストが通ることを確かめる
- [x] 2.4 `internal/permissions` に失敗するテストを書く。`Check` が、設定ファイルと同じ誤りを断ることを確かめる。加えて断る規則は、`Bash(git config *)`、`Bash(git -c *)`、`Bash(git *)`、`Bash(git:*)`、`Bash(*)`、201文字の規則である。`Bash(git mv *)` と `Skill` は通すことも確かめる。実装して通す
- [x] 2.5 `internal/permissions` に失敗するテストを書く。`Candidates` が、`git mv a b` から2つの候補を、`mkdir -p x` から `mkdir` の候補を、Bashでないツールからツールの名前を作ることを確かめる。つないだコマンド（`|`、`&&`、`;`、`for`、`cd`、`$(`）と `git -c` には候補を作らないことも確かめる。実装して通す

## 3. エージェントが許可のファイルを使う

- [x] 3.1 `internal/agent` に失敗するテストを書く。全体、プロジェクト、issueの許可のファイルがあるとき、実装の実行と直す実行の許可に、`allow_tools_extra` のあとにその順で入ることを確かめる。別のissueの許可が入らないこと、計画の実行に入らないこと、`Check` に通らない手書きの規則が入らずログに1回だけ残ることも確かめる
- [x] 3.2 `settings` の呼び出しに許可のファイルの規則を渡す（D4）。3.1のテストと、`go test ./internal/agent` が通ることを確かめる

## 4. 受信箱のAPI

- [x] 4.1 `internal/inbox` に失敗するテストを書く。最新のpermission待ちのコメントから、日本語と英語のどちらの見出しでも拒否の一覧と候補が詳細に入ることを確かめる。エージェント以外が書いた同じ形のコメントを読まないこと、`deniedList` が作る本文と往復できることも確かめる。D5のとおり実装して通す
- [x] 4.2 `internal/inboxserve` に失敗するテストを書く。`GET /v1/permissions` が全体、各プロジェクト、指定したissueの規則と端末の名前を返すことを確かめる。`POST /v1/permissions` が追加と削除をし、断られる規則が混じれば何も書かずに理由を返すことも確かめる。設定に無いプロジェクトと存在しないissueを断ること、`inbox.auth` のある構成で管理者でない人に403を返すこと、監査ログに `web.action` と保存の出来事が残ることも確かめる
- [x] 4.3 `inboxapi` に型を足し、`internal/inboxserve` に口を足す（D6）。4.2のテストが通ることを確かめる
- [x] 4.4 `web/internal/server` に失敗するテストを書く。`/api/permissions` が受信箱のサービスへ中継されること、POSTの本文がJSONでないときと `Origin` が合わないときに403を返すことを確かめる。実装して通す

## 5. Web表示

- [x] 5.1 `web/ui` に失敗するテストを書く。permission待ちの1件の画面に、拒否の一覧、候補の選択と編集、範囲の選択（既定はこのissue）、「許可して再実行」と「許可だけ」が出ることを確かめる。「許可して再実行」を押すと、許可の保存のあとに再実行の文が1回投稿されることを確かめる。保存に失敗したときは投稿しないことも確かめる。候補の無い操作では押せないこと、断られた理由が規則の近くに出ることも確かめる。英語の表示も確かめる
- [x] 5.2 5.1の部品を `IssuePane` に足し、`defineMessages` に日本語と英語の文言を書く（D7）。5.1のテストが通ることを確かめる
- [x] 5.3 `web/ui` に失敗するテストを書く。1件の画面にissueの許可の一覧と外すボタンが出ること、設定の画面に全体とプロジェクトの許可の節と端末の名前が出て、足せて外せることを確かめる。管理者でない人には操作の部品が出ず、`/api/permissions` を呼ばないことも確かめる。実装して通す

## 6. 記録と文書

- [x] 6.1 `docs/DECISIONS.md` にADR-076を足す。ADR-045の「受信箱のAPIとWeb表示からは変えられない」を覆すことと、決めたこと（既定のgit、置き場所、権限、検査、画面、寿命、端末の制限）を書く。理由に2026-11-15のpermission待ちの件数を書く。ADR-075と同じ形であることを読んで確かめる
- [x] 6.2 `docs/ARCHITECTURE.md` の実行の許可の記述に、既定のgitと `permissions/` を足す。`docs/GUIDE.md` と `docs/GUIDE.ja.md` に、Web表示から許可を足す手順と、別の端末に届かないことを書く。日本語と英語の版は同じ内容にする。`docs/SECURITY.md` と `docs/SECURITY.ja.md` に、WUIから足せる許可と断る規則を書く。両方の版を読み比べて確かめる
- [x] 6.3 `make verify` を、sandboxの外で実行して通ることを確かめる

## 7. 本番での確認

- [ ] 7.1 `switchboard agent run`、受信箱のサービス、Web表示を、新しいバイナリで起動し直す。permission待ちのissueをWeb表示で開き、候補を選んで「許可して再実行」を押す。許可のファイルに規則が書かれ、再開した実行の `run.started` の `allow` にその規則が入り、同じコマンドが拒否されないことを、監査ログで確かめる
