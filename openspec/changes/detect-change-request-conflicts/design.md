# Design

## Context

動機はproposal.mdの「Why」を参照。今の見張りは `internal/agent/ciwatch.go` にある。

- `scanWatches` は、`reconcile_interval` ごとにdoneのissueを検索する。`findWatch` は、最新の完了のマーカーに `sha` と `cr` があり、pushから6時間（`ciWatchFor`）以内のissueだけを `watches` に入れる
- `checkWatch` は、`poll_interval` ごとに `ChangeRequestHead` で変更要求を1回読む。変更要求が閉じたとき、headが変わったとき、CIが通ったとき、CIが10分現れないとき、6時間で終わらないときに `unwatch` する
- `ciFailed` は、`fixPlan` で回数を数え、上限の内なら `launch` で直す実行を始める。上限を超えたら `giveUp` の待ちを書く
- 直す実行は `Prepare` でbaseをfetchし、システムプロンプトには「baseと衝突したら `git merge origin/<base>` で解消する」という規則が既にある（ADR-042）
- `ChangeRequestHead` は、GitHubの `GET /pulls/{n}` とGitLabの `GET /merge_requests/{n}` を読む。どちらの応答にも衝突の情報が載っているが、今は読み捨てている

## Goals / Non-Goals

**Goals:**

- 1つの見張りで、CIの結果と変更要求の衝突の両方を判定する
- 衝突の判定のために、フォージへの問い合わせを増やさない（CIの見張りの間は、既に読んでいる変更要求の応答を使う）
- 再起動しても、マーカーから同じ見張りに戻れる

**Non-Goals:**

- baseのheadの変化を見張って問い合わせを減らすこと
- `behind` や `blocked` の変更要求への対処
- 人がpushした後の衝突を直すこと

## Decisions

### 見張りを2つの段に分ける

`watch` に「CIの段が終わった」ことを表す項目と、最後に衝突を問い合わせた時刻を足す。

- CIの段：今と同じく `poll_interval` ごとに読む。CIが通ったとき、CIが10分現れないとき、6時間で終わらないときは、`unwatch` せずに衝突の段へ移る
- 衝突の段：`reconcile_interval` ごとに `ChangeRequestHead` だけを読み、CIは読まない
- どちらの段でも、変更要求が閉じたとき、headが `sha` と違うとき、issueが `agent:done` でなくなったときは `unwatch` する

`findWatch` から `ciWatchFor` の条件を外す。pushから6時間を過ぎたissueは、最初から衝突の段で見張る。

別の見張りの表（`conflictWatches`）を作る案は採らない。同じ変更要求を2つの表から読むと問い合わせが倍になり、`unwatch` の理由も2か所で揃える必要が出るからである。

### 衝突の判定を変更要求の状態に入れる

`forge.ChangeRequestState` に `Conflict bool` を足す。

- GitHub：`mergeable` を `*bool`、`mergeable_state` を文字列で読む。`mergeable` が `false` で、`mergeable_state` が `dirty` のときだけ `Conflict` を真にする。`null`（計算中）は偽とし、次の問い合わせで改めて読む。GitHubは `GET /pulls/{n}` をきっかけに計算を始めるので、次の問い合わせには値が入っている
- GitLab：`has_conflicts` を読む。`detailed_merge_status` が `checking` のときは偽とする

`CI` のインターフェースは変えない。`ChangeRequestHead` の名前は残し、doc commentに衝突の有無も返すと書く。

### 衝突をCIより先に見る

`checkWatch` は、変更要求の状態を読んだ直後、CIを読む前に `Conflict` を見る。真なら `conflicted` を呼び、CIは読まずに戻る。衝突を直す実行のpushの後で、新しい `sha` のCIを改めて見張る（Q6）。

### 衝突を直す実行は `ciFailed` と同じ形にする

`conflicted` は `ciFailed` と同じ順で進む。

1. `fixPlan`（want=`agent:done`）で回数を数える
2. 上限を超えていれば、`giveUp` に `waiting(reason=conflict)` の待ちを入れて書く
3. 枠が無ければ次の問い合わせに回す
4. `launch` する。始まったら `unwatch` する

- `fixInfo` の `cause` に `<prompt.FixConflict>`（`"conflict"`）を足す。`report` には変更要求のURLとbaseの名前だけを書く。フォージから読んだ文は入れない
- 着手のコメントは `<msg.fixConflict>`（「baseとの衝突を解消します（1/2）」）にする
- `FixSystem` に `FixConflict` の分岐を足す。`git merge origin/<base>` で解消し、checksを走らせてコミットすること、矛盾して決められないときは `waiting` で質問することを指示する
- 許可ツールの `git merge` は、ADR-042で実装する実行のすべてに入っているので、足す物は無い。specの「直す実行で許すgitの操作」を実態に合わせて書き直すだけである

### 待ちのreason

`marker` に `Conflict = "conflict"` を足す。`giveUp` のコメントには、変更要求の番号、直した回数、`fixSummaries` の各回の要約を書く。ログは無い。WUIは `isFailure` に `conflict` を足し、`IssuePane` に「返信するとエージェントが実行を再開し、baseとの衝突を自動で直す回数を数え直します」の案内を出す。

### 記録

`docs/DECISIONS.md` にADR-044を足す。ADR-041のきっかけに衝突を加えること、見張りを変更要求がopenの間に延ばすこと、トークンの権限が増えないことを書く。`docs/SECURITY.md` の4章の、人のきっかけ無しに始まる実行の列挙に衝突を加える。

## Risks / Trade-offs

- doneのissueが多いと、`reconcile_interval` ごとの問い合わせが増える。対策：1件1リクエストで、closeされるか変更要求が閉じれば終わる。問題になれば、baseのheadの変化をきっかけにする（後回しにしたもの）
- mergeを直してpushする間にbaseがまた進み、衝突が繰り返す。対策：`agent.max_fix_attempts` で止まり、`waiting(reason=conflict)` になる
- GitHubの `mergeable` が計算中のまま返り続ける。対策：偽として扱い続けるので、直す実行は始まらない。誤って直すより安全側である
- mergeの解消が意味の上で誤る。対策：直す実行の終わりは、ほかの実行と同じく完了の検証、出力検査、CIの見張りを通る。PRはdraftのまま残る

## Migration Plan

設定とマーカーの形は変わらない。新しいエージェントを起動すると、既存の `agent:done` のissue（マーカーに `sha` と `cr` があるもの）が衝突の段で見張りに入る。PR #71もこのとき見張りの対象に入る。戻すときは前のバイナリに替えるだけでよく、`waiting(reason=conflict)` のissueは前のWUIでもreasonの文字のまま表示される。
