# Design

## Context

動機はproposal.mdのWhyを参照する。決定の出典はgrilling.mdである。

実行の許可は `internal/agent/run.go` の `settings` が組み立てる。ファイルを書く実行の許可は `baseAllow` に、計画の実行の許可は `internal/agent/planrun.go` の `planBaseAllow` にある。`claudecode.Commands` は、`git log` から `Bash(git log)` と `Bash(git log *)` の2つの規則を作る。gitは `gitUnsandboxed` でsandboxの外に出すので、許可の規則に無いサブコマンドは `dontAsk` で拒否される。sandboxの中のコマンドは `autoAllowBashIfSandboxed` で規則なしに動くので、足す規則が意味を持つのは、gitと検証コマンドと、Bash以外のツールである。

設定ファイルの `allow_tools_extra` は、`internal/config` の `checkAllowTools` で検査する。追加の指示は `internal/instructions` が設定のディレクトリの `instructions/` に置き、受信箱のサービスの `/v1/instructions` で読み書きする。`inbox.auth` のある構成では `adminOnly` で管理者に絞っている。

permission待ちのコメントの「拒否された操作」の節は、`deniedList` が作る。各行は `- Bash: \`<コマンド>\`` の形である。コマンドの中のバッククォートは `'` に置き換えてある。

## Goals / Non-Goals

**Goals:**

- 調べるためのgitと、worktreeの中のファイルの移動で、人の返信を待たない。
- permission待ちを、スマホのWeb表示から、許可の追加と再実行の1回の操作で解く。

**Non-Goals:**

- CLIとAndroidのアプリからの編集（grilling.mdのQ12）。
- 別の端末のエージェントへ許可を届けること（Q11）。
- つないだコマンドを規則で許すこと。規則では当たらないので、画面でそう伝えるだけにする。

## Decisions

### D1. 既定のgitは2つの一覧にまとめる

`internal/agent` に、読むだけのサブコマンドの一覧 `gitRead` と、worktreeの中だけを変える一覧 `gitWorktree` を置く。`baseAllow` は今の6つに両方を足し、`planBaseAllow` は今の3つに `gitRead` を足す。どちらも `claudecode.Commands` で引数ありと無しの規則にする。計画の実行の拒否の一覧は変えない。実行のあとの検査（ADR-038）が、計画の実行の変更を捨てる防壁であり続ける。

代わりの案：`Bash(git *)` を許して、危ないものだけを拒否に並べる案。拒否の一覧から漏れたサブコマンドが、sandboxの外で動くことになる。許すものを並べれば、漏れたサブコマンドは拒否される。

### D2. fetch済みの案内はsystem promptの組み込みの節に書く

すべての実行のsystem promptに、`origin/<base>` は実行の前にfetch済みであることと、`git fetch` は許されないことを1文で書く。baseの名前はプロジェクトの `base_branch` から入れる。system promptは英語だけで書く今の扱いに従う。

### D3. 許可のファイルは新しいパッケージ `internal/permissions` が扱う

`internal/instructions` と同じ形の `Store{Dir, Audit}` を作る。`For(configPath)` は設定のディレクトリの `permissions/` を指す。範囲は `Scope{Project string; Issue int}` で表し、両方空なら全体とする。

- `Read(scope) []string`：ファイルを読み、空行と `#` の行を除き、重複をまとめる。ファイルが無ければ空。
- `Add(scope, rules)` と `Remove(scope, rules)`：同じディレクトリの一時ファイルに書いてから置き換える。受信箱のサービスの中では1つのmutexで直列にする。手で書いた行とコメントは残し、足す規則は末尾に足し、外す規則は一致する行だけを消す。
- `Check(rule) error`：`internal/config` の検査を、1つの規則を調べる関数（`<config.CheckAllowRule>`）として公開して使う。その上に、WUIだけの検査を重ねる。対象は `git config`、`git -c`、サブコマンドを限らないgit、ワイルドカードだけの規則、200文字を超える規則である。
- `Candidates(tool, command) (rules []string, compound bool)`：拒否から候補を作る。コマンドに `|`、`&`、`;`、`<`、`>`、`$(`、改行のどれかがあるか、先頭の語が `for`、`while`、`if`、`cd` なら、`compound` とする。gitなら先頭の2語から `claudecode.Commands("git <sub>")` で候補を作る。`git -c` のように2語目が `-` で始まるものは候補にしない。それ以外は先頭の1語から作る。最後に、`Check` に通らない候補を除く。

### D4. エージェントは実行のたびに3つのファイルを読む

`settings` の `extra` に、設定の2つの `allow_tools_extra` に続けて、`permissions` の全体、プロジェクト、issueの規則を渡す。`Check` に通らない規則は渡さず、同じ規則のログはプロセスの間に1回だけ残す。`--tools` に渡すツールの名前は、今の `extra` の扱いがそのまま集める。計画の実行の組み立て（`planrun.go`）には渡さない。実行の開始の記録（ADR-045の `run.started` の `allow`）には、足した規則が今までどおり入る。

### D5. 拒否は受信箱が詳細を作るときにコメントから読む

`internal/inbox` の1件の詳細で、エージェントの最新の非一時的なマーカーが `waiting` で `reason=permission` のとき、そのコメントの本文から拒否を読む。投稿者がエージェントの身元であることを、マーカーを読む今の処理と同じく確かめる。節の見出しは、日本語の `**拒否された操作**` と英語の `**Refused operations**` の両方を認める。見出しの次の行から空行までの、`- ` で始まる行を読む。コマンドのある行からはツールとコマンドを、ツールだけの行からはツールの名前を取る。各行に `permissions.Candidates` をかけて、詳細の `Denials` に入れる。

代わりの案：エージェントの実行のディレクトリから読む案。受信箱のサービスとエージェントが別の端末だと読めない。

### D6. APIは `/v1/permissions` の1つの口にまとめる

- `GET /v1/permissions?issue=<ref>`：全体、設定の各プロジェクト、指定したissueの規則と、端末の名前を返す。
- `POST /v1/permissions`：`{scope: "global"|"project"|"issue", ref, add: [...], remove: [...]}`。`ref` はプロジェクトかissueの参照。検査に1つでも落ちれば何も書かず、`invalid` の誤りに規則ごとの理由を付けて返す。issueは、フォージで存在を確かめてから書く。

どちらも `inbox.auth` のある構成では `adminOnly` を通す。保存のたびに `web.action` と `<permission.saved>` か `<permission.removed>` を監査ログに残す。Web表示の `web/internal/server` は `/api/permissions` として中継し、POSTには指示の保存と同じJSONと `Origin`、`Host` の検査をかける。

### D7. 画面は既存の部品の横に足す

- 1件の画面（`IssuePane`）：理由が `permission` のとき、返答の欄の上に「拒否された操作」の箱を出す。候補ごとにチェックと編集の欄、範囲の選択（既定はこのissue）、「許可して再実行」と「許可だけ」を置く。「許可して再実行」は、`POST /api/permissions` が成功してから、今の「再実行する」と同じ文で返答のPOSTを送る。
- 1件の画面のissueの許可：規則があるときだけ、詳細の下に一覧と外すボタンを出す。
- 設定の画面：追加の指示の節の下に許可の節を置く。全体と各プロジェクトに一覧と足す欄を置き、端末の名前を出す。
- 文言は各部品の `defineMessages` に日本語と英語で書く。管理者でない人には、操作の部品を出さない。

## Risks / Trade-offs

- ［WUIから足した規則で、sandboxの外のgitが広がる］→`config` と `-c`、サブコマンドを限らないgitを断る。足せるのは、`inbox.auth` のある構成では管理者だけで、監査ログに誰が何を足したかが残る。認証の無い構成でLANの誰でも足せることは、追加の指示と同じく受け入れる（ADR-036）。
- ［候補の規則が、拒否された引数よりも広い］→候補はサブコマンド単位で、保存の前に人が見て編集できる。範囲の既定をこのissueにして、広げるのは人が選んだときだけにする。
- ［拒否の節の形が変わると、候補が出なくなる］→節を作る `deniedList` と読む側に、往復のテストを置く。読めなくても、待ちは今までどおり返答で解ける。
- ［別の端末のエージェントに許可が届かない］→設定の画面に、どの端末の許可かと、その端末のエージェントにだけ渡ることを出す（grilling.mdのQ11）。

## Migration Plan

設定ファイルとトークンは変わらない。`permissions/` は最初の保存で作る。エージェントと受信箱のサービスとWeb表示を入れ替えるだけでよい。戻すときは前のバイナリに戻す。`permissions/` のファイルは古いエージェントからは読まれず、害は無い。
