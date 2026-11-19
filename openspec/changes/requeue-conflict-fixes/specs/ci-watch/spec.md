# Spec Delta

## MODIFIED Requirements

### Requirement: 衝突を直す回数の上限に達したとき
変更要求がbaseと衝突し、この巡で直した回数が上限に達し、順番待ちに入り直した回数も上限に達しているとき、エージェントは、issueを `waiting(reason=conflict)` にしなければならない（MUST）。
上限はどちらも `<agent.max_fix_attempts>` で、このとき直す実行は始めない。
そのコメントには、変更要求の番号と、自動で直した回数と各回の要約を書かなければならない（MUST）。入り直したことがあるときは、入り直した巡の数も書く。

#### Scenario: 2回直しても衝突する
- **WHEN** `<agent.max_fix_attempts>` が2の設定で、順番待ちに2回入り直し、3巡目の2回の直しの後でPRがまた衝突する
- **THEN** 直す実行は始まらず、issueは `agent:waiting` になり、マーカーの `reason` は `conflict` である

#### Scenario: 入り直しても衝突が続く
- **WHEN** `<agent.max_fix_attempts>` が1の設定で、issue 12のPRが衝突し、1回直したあと順番待ちに入り直す。再開した直しの後でPRがまた衝突する
- **THEN** 直す実行は始まらず、issueは `agent:waiting` になり、マーカーの `reason` は `conflict` である。コメントには、1巡入り直しても解けなかったことが書いてある

#### Scenario: 自動では直さない設定
- **WHEN** `<agent.max_fix_attempts>` が0の設定で、PRが衝突する
- **THEN** 直す実行は始まらず、順番待ちにも入らず、issueは `waiting(reason=conflict)` になる

## ADDED Requirements

### Requirement: 衝突の直しの順番待ちへの入り直し
変更要求がbaseと衝突し、この巡で直した回数が `<agent.max_fix_attempts>` に達し、順番待ちに入り直した回数が `<agent.max_fix_attempts>` 未満のとき、エージェントは、issueを順番待ちに入り直させなければならない（MUST）。
入り直すとき、エージェントは、`state=queued reason=conflict` で `run=` を持つマーカーの順番待ちのコメントを投稿し、issueを `agent:ready` に変えなければならない（MUST）。
入り直したissueは、順番待ちの最後尾に入る。順番待ちに入り直した後、エージェントは、ほかに順番待ちのコメントを投稿してはならない（MUST NOT）。

#### Scenario: 上限を超えて入り直す
- **WHEN** `<agent.max_fix_attempts>` が1の設定で、issue 12のPRが衝突し、1回直した後でまた衝突する
- **THEN** issue 12は `agent:ready` になり、`state=queued reason=conflict` で `run=` を持つマーカーのコメントが付く。`agent:waiting` にはならない

#### Scenario: ほかのissueが順番を待っている
- **WHEN** `<agent.max_parallel>` が1の端末で別の実行が動き、issue 13が順番待ちにある。そこでissue 12が入り直す
- **THEN** 動いている実行が終わると、先にissue 13が始まり、issue 12の衝突を直す実行はその後に始まる

### Requirement: 入り直した衝突の直しの再開
エージェントが付けた `agent:ready` と、`reason=conflict` の順番待ちのコメントを持つissueの順番が来たとき、エージェントは、衝突を直す実行を始めなければならない（MUST）。計画の実行や実装の実行は始めない。
そのとき、エージェントは、変更要求がまだ衝突しているかを確かめずに始め、順番待ちのコメントを消さなければならない（MUST）。
着手のマーカーは `reason=fix` と `requeue=<巡の番号>` を持ち、着手のコメントには、順番待ちから再開したことと巡の番号を書かなければならない（MUST）。
エージェントが再起動しても、エージェントは、順番待ちのコメントから同じように再開しなければならない（MUST）。

#### Scenario: 順番が来る
- **WHEN** issue 12が衝突の直しで順番待ちに入り直し、スロットが空く
- **THEN** issue 12は `agent:running` になり、`reason=fix requeue=1` のマーカーの着手のコメントが付き、衝突を直す実行が始まる。順番待ちのコメントは消える

#### Scenario: 再起動の後
- **WHEN** issue 12が衝突の直しで順番待ちに入り直した後に、エージェントが再起動する
- **THEN** 再起動したエージェントは、順番待ちのコメントを消さずに残し、スロットが空くとissue 12の衝突を直す実行を始める

### Requirement: 入り直した後の直す回数
エージェントは、`requeue=` を持つ着手から後の直す実行だけを、この巡で直した回数として数えなければならない（MUST）。`reason=conflict` の順番待ちのコメントから後も、同じく数え直す。
エージェントは、信頼済みの人のコメントか、`mode` を持つ着手から後にある、`requeue=` を持つ着手の数を、順番待ちに入り直した回数として数えなければならない（MUST）。

#### Scenario: 再開した直しの回数
- **WHEN** `<agent.max_fix_attempts>` が2の設定で、2回直した後に順番待ちに入り直し、順番が来る
- **THEN** 着手のコメントには「（1/2）」と書いてある

#### Scenario: 人の返答で数え直す
- **WHEN** issue 12が入り直しの上限に達して `waiting(reason=conflict)` になり、信頼済みの人が返答する
- **THEN** 直す回数と入り直した回数は、どちらも0から数え直す

### Requirement: CIの失敗とpushの拒否は入り直さない
CIの失敗かpushの拒否で、直す回数が上限に達したとき、エージェントは、issueを順番待ちに入り直させてはならない（MUST NOT）。今までどおり待ちにする。

#### Scenario: CIの失敗が上限を超える
- **WHEN** `<agent.max_fix_attempts>` が1の設定で、1回直した後のCIもまた失敗する
- **THEN** issueは `waiting(reason=ci)` になり、順番待ちのコメントは付かない
