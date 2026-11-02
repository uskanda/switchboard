# Design

## Context

実行の種類ごとの許可とsystem promptは、`internal/agent/run.go` の `work` で組み立てる。
足す許可は `settings()` が既定のallowのあとに重なりを省いてつなぐ。
指示は `internal/instructions` の `Plan`/`Run` が、全体とプロジェクトのファイルを前後の空白を除いてつなぐ。
`instructions.saved` は、保存した中身そのもののSHA-256を記録している。

## Goals / Non-Goals

**Goals:**
- 監査ログの1行から、その実行の許可と指示の版を特定できる
- system promptに入れた指示と、ログのハッシュが、同じ読み込みから来る

**Non-Goals:**
- 既定のallowやdenyを記録すること（コードの版で決まる）
- claudeを起動しない実行（開始ラベルの衝突、worktreeの準備の失敗）を記録すること

## Decisions

- `instructions.Store.Load(kind, project) Loaded` を足す。`Loaded{Text, Global, Project}` で、`Text` は今の `Plan`/`Run` と同じつなぎ方、`Global`/`Project` はファイルの中身のハッシュ。`Plan`/`Run` は `Load` の結果の `Text` を返す。ハッシュは `Save` と共通の小さな関数で計算する。読み込みを2回に分けると、その間の保存でログとpromptが食い違うので、1回にまとめる。
- `settings()` から既定のallowを `baseAllow` に、足す許可の計算を `extraAllow` に切り出し、`settings()` とログの両方がその結果を使う。ログの一覧と実際のallowがずれない。`extraAllow` は空でもnilを返さず、ログに `[]` が残る。
- `run.started` は `work` で `claudecode.Run` の直前に残す。fixは、最初の実行と `fixAgain` 経由の実行のどちらも `work` を通るので、両方が記録される。
- 種類の名前は既存の `runKind` の `String` を使う。新しい対応表は作らない。

## Risks / Trade-offs

- ファイルを読めないときのハッシュが空になり、ファイルが無いときと区別できない → system promptでも読めないファイルは外しているので、実行への入力としては同じである。
