# Proposal

## Why

ADR-045で、ファイルを書く実行に `allow_tools_extra` と実装用の指示（`run.md`）を足せるようにした。
設定と指示は書き換えられるので、ある実行がどの許可とどの指示で動いたかを、あとから確かめられない。
実行の結果がおかしいときに原因を切り分けられるよう、実行ごとに使った許可と指示の版を監査ログに残す。

## What Changes

- claudeを起動する直前に、監査ログへ `run.started` を1行残す。中身は、issue、実行のID、実行の種類（fixのときは原因も）、足した許可の一覧、指示のファイルごとのSHA-256である。
- 指示のハッシュは `instructions.saved` の `sha256` と同じ式で計算する。同じ値を持つ `instructions.saved` の行から、どの保存の版で動いたかが分かる。
- 指示の内容そのものは監査ログに書かない。
- 計画の実行では、足した許可を `[]` にし、ハッシュは計画用の指示（`plan.md`）のものにする。

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

- `run-instructions`: 実行を始めるときに、使った追加の許可と指示のハッシュを監査ログに残す要件を足す

## Impact

- `internal/instructions`：1回の読み込みで、つないだ文字列とファイルごとのハッシュを返す関数を足す
- `internal/agent/run.go`：足した許可の計算を関数に切り出し、`work` で `run.started` を残す
- `README.md`：追加の指示の節に、監査ログの記録を書き足す
- 不変条件は変わらない。新しいADRは作らない
