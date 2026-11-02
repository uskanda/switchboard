# Proposal

## Why

エージェントが作ったPR/MRは、`agent:done` の後でbaseに別の変更が入ると、baseと衝突することがある。GitHubは衝突のあるPRで `pull_request` のCIを走らせないので、今のCIの見張りは「CIが現れない」として黙って終わり、人が気付いて返信するまで止まる。PR #71（issue #67）がその例である。直す実行の仕組み（ADR-041）とbaseとの衝突をmergeで解消する規則（ADR-042）は既にあるので、きっかけに衝突を足せば人の手を待たずに直せる。

## What Changes

- エージェントは、openで `agent:done` のissueの変更要求を、変更要求がopenの間ずっと見張り、`reconcile_interval` ごとにbaseとの衝突を問い合わせる。pushから6時間というCIの見張りの期限は、衝突の見張りには使わない
- 衝突とみなすのは、GitHubの `mergeable=false` かつ `mergeable_state=dirty`、GitLabの `has_conflicts=true` だけとする。計算中（`mergeable=null` など）は次の問い合わせに回す。`behind` と `blocked` は対象外
- 変更要求のheadがエージェントの最新の完了のマーカーの `sha` のままで衝突しているとき、エージェントは `agent:running` に戻し、baseとの衝突を解消する直す実行を始める。headが別のコミットなら直さない
- 衝突を直す実行は、CIの失敗、non-fast-forwardと同じ `agent.max_fix_attempts` で数える。上限に達したら、新しい `waiting(reason=conflict)` にする
- CIの失敗と衝突が同時にあるときは、衝突を先に直す
- 直す実行のプロンプトに、衝突を解消する原因（`conflict`）を足す
- 受信箱とWUIで、reasonの `conflict` を表示用の言葉にする
- ADR-043を足し、SECURITY.mdの4章（人のきっかけ無しに始まる実行）を書き換える

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

- `ci-watch`: 変更要求の衝突の見張り、衝突の判定、衝突を直す実行の開始、上限に達したときの `waiting(reason=conflict)`、CIの失敗との順序の要求を足す
- `run-outcome`: 自動で直す回数に、衝突を直す実行を含める
- `agent-run`: 直す実行で `git merge` を許す対象に、衝突を直す実行を含める
- `config`: `<agent.max_fix_attempts>` が0のとき、衝突も自動では直さない

## Impact

- `internal/forge`: `ChangeRequestState` に衝突の有無を足し、GitHubとGitLabのクライアントでそれを読む
- `internal/agent`: `ciwatch.go` の見張りの期間と判定、`fix.go` の直す実行の原因、`messages.go` の着手と待ちの文
- `internal/prompt`: `FixConflict` と、その直す実行のシステムプロンプト
- `internal/marker`: 待ちのreasonに `conflict` を足す
- `web/ui`: reasonの `conflict` を失敗の一種として表示し、返信の案内を出す
- `docs/DECISIONS.md`（ADR-043）、`docs/SECURITY.md`
- フォージへの問い合わせ：doneのissue1件につき、`reconcile_interval` ごとに変更要求を1回読む。トークンの権限は増えない
