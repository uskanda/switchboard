# Proposal

## Why

2026-11-15に、`gh:uskanda/smarternews` で `waiting(reason=permission)` が8件出た。拒否されたのは主に `git ls-tree`、`git fetch`、`git mv`、`git rm`、`git merge-base` である。gitはsandboxの外で動かすので（ADR-057）、許可の規則に無いサブコマンドはすべて拒否され、読むだけの操作でも人の返信を待つ。
拒否されたときに許可を足すには、端末で `config.yaml` の `allow_tools_extra` を書き換える必要がある。ADR-045は、許可を受信箱のAPIとWeb表示から変えられないと決めている。そのため、スマホから待ちを解けない。

## What Changes

- ファイルを書く実行（実装と直す実行）が既定で許すgitを広げる。読むだけのものを足す。対象は `ls-tree`、`ls-files`、`show`、`cat-file`、`merge-base`、`rev-parse`、`rev-list`、`blame`、`grep`、`shortlog`、`describe` である。worktreeの中だけを変える `mv`、`rm`、`restore`、`stash` も足す。計画の実行には読むだけのものを足す。
- `git fetch` は今までどおり拒否する。system promptに、`origin/<base>` は実行の前にfetch済みだと書く。
- 許可の規則を、設定のディレクトリの `permissions/` の下にファイルとして置けるようにする。単位は全体、プロジェクト、issueの3つとする。ファイルを書く実行のたびに読み、`allow_tools_extra` のあとにつなげる。計画の実行には渡さない。
- 受信箱のAPIに、許可の読み出し、追加、削除の口を足す。`inbox.auth` のある構成ではadminだけが使える。検査は設定ファイルと同じものに加え、`git config`、`git -c`、サブコマンドを限らないgit、ワイルドカードだけの規則を断る。
- Web表示に3つの入口を足す。
  - permission待ちの1件の画面：拒否された操作と規則の候補を出し、範囲を選んで「許可して再実行」か「許可だけ」を押せる。
  - 設定の画面：全体とプロジェクトの許可の一覧と編集。
  - 1件の画面：issue単位の許可の一覧と外すボタン。
- **BREAKING**（決定の変更）：ADR-045の「許可は受信箱のAPIとWeb表示からは変えられない」を覆す。新しいADR-076に記録する。

## Capabilities

### New Capabilities

- `permission-grants`: 許可のファイルの置き場所と形、読み込み、WUIから足す規則の検査、拒否から候補を作る規則。

### Modified Capabilities

- `agent-run`: ファイルを書く実行が既定で許すgitを広げ、fetch済みの案内を足す。追加の許可に `permissions/` のファイルの規則を加える。
- `plan-mode`: 計画の実行で許すgitに、読むだけのものを足す。
- `inbox-api`: 許可の読み出し、追加、削除の口を足す。許可の設定を読み書きする手段を持たないとした要件を改める。
- `web`: permission待ちの許可の操作、設定の画面の許可の節、1件の画面のissue単位の許可の一覧を足す。設定の画面に許可の編集を置かないとした要件を改める。

## Impact

- `internal/agent/run.go` の `baseAllow` と `settings`。`internal/agent/planrun.go` の `planBaseAllow`。system promptの組み立て。
- 新しいパッケージ `internal/permissions`（ファイルの読み書きと検査）。`internal/config` の `checkAllowTools` を共有する。
- `internal/inbox`：permission待ちのコメントから拒否を読む。`internal/inboxserve` と `inboxapi`：新しい口。`web/internal/server`：中継とPOSTの検査。
- `web/ui`（1件の画面、設定の画面、日本語と英語の文言）。
- `docs/DECISIONS.md`（ADR-076）と `docs/ARCHITECTURE.md`。利用者向けの `docs/GUIDE.md`、`docs/SECURITY.md` と、その日本語の版。
- フォージのトークンの権限と、`config.yaml` の形は変わらない。
