## ADDED Requirements

### Requirement: 衝突の見張りの対象
見張る対象は、担当プロジェクトのopenで `agent:done` のissueのうち、最新の完了のコメントのマーカーに `sha` と `cr` があるものとする。
その変更要求がopenの間、エージェントは、変更要求とbaseの衝突を見張らなければならない（MUST）。
衝突の見張りには、pushからの経過時間の期限を設けてはならない（MUST NOT）。

#### Scenario: CIの見張りが終わった後
- **WHEN** issue 12のPRのCIが通ってCIの見張りが終わり、その2日後にbaseへ別のPRがマージされてissue 12のPRが衝突する
- **THEN** エージェントは衝突を見つけ、直す実行を始める

#### Scenario: CIを持たないリポジトリ
- **WHEN** CIの無いリポジトリで、pushから10分たってCIの見張りが終わる
- **THEN** エージェントは、そのissueの衝突の見張りを続ける

### Requirement: 衝突の問い合わせの間隔
衝突を見張っているissueがある間、エージェントは、`reconcile_interval` ごとに、その変更要求の状態をフォージに問い合わせなければならない（MUST）。

#### Scenario: doneのissueが3件ある
- **WHEN** 変更要求がopenのdoneのissueが3件ある
- **THEN** エージェントは、`reconcile_interval` ごとに、3つの変更要求を1回ずつ読む

### Requirement: 衝突の見張りの終了
次のどれかのとき、エージェントは、そのissueの衝突の見張りをやめなければならない（MUST）。
変更要求が閉じられたかマージされたとき、issueが閉じられたか `agent:done` でなくなったときが該当する。

#### Scenario: PRがマージされた
- **WHEN** 人がissue 12のPRをマージする
- **THEN** エージェントはissue 12の衝突の見張りをやめ、issueに何も書かない

### Requirement: 衝突の判定
エージェントは、`mergeable` が `false` で `mergeable_state` が `dirty` のGitHubのPRを、衝突していると判定しなければならない（MUST）。
エージェントは、`has_conflicts` が `true` のGitLabのMRを、衝突していると判定しなければならない（MUST）。
フォージが衝突の有無をまだ計算していないとき、エージェントは判定せず、次の問い合わせで判定しなければならない（MUST）。
`mergeable_state` が `behind` か `blocked` のPRを、エージェントは衝突と判定してはならない（MUST NOT）。

#### Scenario: 計算中
- **WHEN** GitHubが、issue 12のPRの `mergeable` に `null` を返す
- **THEN** エージェントは直す実行を始めず、次の問い合わせで改めて判定する

#### Scenario: baseより遅れているだけ
- **WHEN** issue 12のPRの `mergeable_state` が `behind` である
- **THEN** エージェントは直す実行を始めず、issueは `agent:done` のままである

### Requirement: 人がpushした後の衝突
変更要求のheadが最新の完了のマーカーの `sha` と違うとき、エージェントは、衝突を直す実行を始めてはならない（MUST NOT）。

#### Scenario: 人が同じブランチにpushした
- **WHEN** 人が `switchboard/issue-12` にコミットをpushし、その後でPRが衝突する
- **THEN** エージェントは直す実行を始めず、issueは `agent:done` のままである

### Requirement: 衝突を直す実行の開始
変更要求がbaseと衝突し、直した回数が `<agent.max_fix_attempts>` 未満のとき、エージェントは、衝突を直す実行を始めなければならない（MUST）。
始める前に、エージェントはissueを `agent:running` に変える。
そのとき、エージェントは、baseとの衝突を直すことと、何回目の直しかを書いた着手のコメントを投稿しなければならない（MUST）。そのマーカーの `reason` は `fix` とする。

#### Scenario: doneの後にbaseが進んで衝突する
- **WHEN** `<agent.max_fix_attempts>` が2の設定で、issue 12のPRがbaseと衝突する
- **THEN** issueは `agent:running` になり、「baseとの衝突を解消します（1/2）」と書いた、`reason=fix` のマーカーのコメントが付き、直す実行が始まる

#### Scenario: 計画のモードのissue
- **WHEN** 計画の承認の後に実装して完了したissueのPRが、baseと衝突する
- **THEN** 計画の実行ではなく、衝突を直す実行が始まる

### Requirement: 衝突を直す回数の上限に達したとき
変更要求がbaseと衝突し、直した回数が上限に達しているとき、エージェントは、issueを `waiting(reason=conflict)` にしなければならない（MUST）。
上限は `<agent.max_fix_attempts>` で、このとき直す実行は始めない。
そのコメントには、変更要求の番号と、自動で直した回数と各回の要約を書かなければならない（MUST）。

#### Scenario: 2回直しても衝突する
- **WHEN** `<agent.max_fix_attempts>` が2の設定で、2回の直しの後でPRがまた衝突する
- **THEN** 直す実行は始まらず、issueは `agent:waiting` になり、マーカーの `reason` は `conflict` である

#### Scenario: 自動では直さない設定
- **WHEN** `<agent.max_fix_attempts>` が0の設定で、PRが衝突する
- **THEN** 直す実行は始まらず、issueは `waiting(reason=conflict)` になる

### Requirement: CIの失敗と衝突が同時にあるとき
CIの失敗と変更要求の衝突を同じ問い合わせで見つけたとき、エージェントは、衝突を直す実行を始めなければならない（MUST）。CIを直す実行は始めない。

#### Scenario: CIが失敗した後で衝突した
- **WHEN** issue 12のPRのCIのjob `test` が失敗し、同じ問い合わせでPRがbaseと衝突している
- **THEN** 衝突を直す実行が1つだけ始まり、その完了の後にpushしたコミットのCIを改めて見張る

## MODIFIED Requirements

### Requirement: CIの無いリポジトリ
pushから10分たってもそのコミットのCIが1つも現れないとき、エージェントはCIの見張りをやめなければならない（MUST）。issueには何も書かない。衝突の見張りは続ける。

#### Scenario: CIを持たないリポジトリ
- **WHEN** ワークフローとpipelineの無いリポジトリで、エージェントがPRを作って完了する
- **THEN** 10分後にCIの見張りが終わり、issueは `agent:done` のまま、コメントも増えない

#### Scenario: 衝突したPR
- **WHEN** PRがbaseと衝突していて、GitHubが `pull_request` のCIを走らせない
- **THEN** CIの見張りは10分後に終わるが、衝突の見張りが衝突を見つけて直す実行を始める
