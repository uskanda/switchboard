# Proposal

## Why

変更要求がbaseと衝突し、直す実行が `agent.max_fix_attempts` 回を超えると、issueは `waiting(reason=conflict)` になり、人が返答するまで止まる。衝突は、ほかのPR/MRのマージでbaseが動くたびに起き直すので、後に回せば解ける見込みがある。人の手を待つ `conflict` の状態には、極力入らないようにしたい（issue #217）。

## What Changes

- 衝突を直す回数が上限を超えたとき、入り直した回数が `agent.max_fix_attempts` 未満なら、issueを `agent:ready` に戻し、`state=queued reason=conflict run=` の順番待ちのコメントを付けて、順番待ちの最後尾に入れる。
- 順番が来たら、PR/MRがまだ衝突しているかは確かめずに、衝突を直す実行を始める。直す回数は1から数え直す。
- 順番待ちから再開した直す実行の着手のマーカーに、新しいキー `requeue=<巡の番号>` を付ける。直す回数の区切りと、入り直した回数の数えに使う。`reason=fix` はそのままにする。
- 入り直しが `agent.max_fix_attempts` 回に達したあとで上限を超えたら、今までどおり `waiting(reason=conflict)` にする。本文に、入り直した巡の数を書く。`max_fix_attempts: 0` では入り直さない。
- CIの失敗とpushの拒否の上限超えは、今までどおり待ちにする。
- 受信箱のWUIは、`reason=conflict` の順番待ちに専用の文を出す。

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- `ci-watch`: 衝突を直す回数の上限に達したときの動きを、順番待ちへの入り直しに変える。入り直しの回数の上限と、再開した直す実行の着手と回数の数え方を足す。
- `web`: 順番待ちの表示に、衝突の直しを順番待ちに回したことを示す文を足す。

## Impact

- `internal/agent`（`ciwatch.go`、`fix.go`、`limit.go`、`agent.go`、`run.go`、`messages.go`）、`internal/marker`（`requeue=` のキー）。
- `web/ui/src/lib/api.ts` の `queueText`、`inboxapi` のコメント。
- 文書：`docs/DECISIONS.md`（ADR-077）、`docs/DESIGN.md`、`docs/GUIDE.md`、`docs/GUIDE.ja.md`、`docs/SPEC.md`。
- 設定項目は増えない。フォージ上のマーカーにキーが1つ増える。古い版のエージェントは知らないキーを読み飛ばす。
