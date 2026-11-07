# Proposal

## Why

`plan_first: true` のプロジェクトでは、`agent:ready` のissueは要件が明確でも必ず質問と計画の承認を往復し、実装が遅れる。要件が明確なときだけ、エージェントの判断でその往復を省けるようにしたい（issue #168）。

## What Changes

- `<projects[].plan_first>` を `always`、`auto`、`never` の3値にする。前の版の `true` は `always`、`false` は `never` として読み、キーが無ければ `never` とする。ほかの値は設定の誤りとする
- `auto` のプロジェクトの `agent:ready` は、新しいモード `auto` で着手する。計画の段階として扱い、周期の最初の計画の実行にだけ、宣言の `status` に `implement` を許す
- `implement` を宣言した計画の実行は、計画の本文と、質問と承認を省いて実装することを書いたコメントを投稿し、待ちにせず、同じ着手のまま実装の実行に移る。実装の実行のsystem promptは、そのコメントを計画として名指しする
- 省いたあとの返答による再開は、直ちに実装する周期と同じに扱う
- `agent:plan`、`agent:go`、`always` と `never` の `agent:ready` の挙動は変えない
- 受信箱のAPIの、プロジェクトの一覧、プロジェクトの追加、issueを作れる先の一覧に、3値の `plan_first_mode` を足す。真偽値の `plan_first` は古いクライアントのために残す
- Web表示のプロジェクトの追加の欄を、3つから選ぶ形にし、初期値を `auto` にする。一覧と新規issueの注記も3値に合わせる

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- `plan-mode`：`agent:ready` のモードの決定に `auto` を足し、計画の段階を省く判断、省いたときのコメント、省いたあとの実装の実行と待ちを足す
- `config`：`<projects[].plan_first>` を3値にし、前の版の真偽値を読み続ける
- `inbox-api`：プロジェクトの一覧、追加、issueを作れる先の一覧で、`plan_first_mode` を扱う
- `web`：プロジェクトの追加の欄を3択にし、一覧と新規issueの注記を3値で表示する

## Impact

- Go：`internal/config`（`PlanFirst` の型、書き込み）、`internal/marker`（`mode=auto`、`reason=plan-skipped`）、`internal/agent`（モードの決定、分類、計画の実行の宣言、省いたあとの実装の実行への引き継ぎ）、`internal/prompt`（省ける計画の実行と、省いたあとの実装の実行のsystem prompt）、`inboxapi`、`internal/inboxserve`
- Web UI：`ProjectsSection`、`NewIssue`、`lib/api.ts`
- 文書：`docs/GUIDE.md`、`docs/DECISIONS.md`（ADR-064）、`docs/examples/openspec.md`、`config/switchboard.example.yaml`
- 互換：`mode=auto` で着手したissueを古い版のエージェントが読むと `direct` として扱う。すべての端末を同じ版にそろえる
