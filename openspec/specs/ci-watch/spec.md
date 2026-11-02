# ci-watch Specification

## Purpose
エージェントが自分でpushしたコミットのCIを見張る。失敗を判定し、失敗の情報をフォージから集めて、直す実行を始めるか、人の判断を待つ。

## Requirements

### Requirement: 見張りの開始
エージェントが変更要求のURLを書いた完了のコメントを投稿したとき、エージェントは、そのコメントのマーカーの `sha` のコミットのCIを見張らなければならない（MUST）。

#### Scenario: PRを作って完了する
- **WHEN** issue 12の実行が完了し、エージェントがコミット `abc123` をpushしてPR #48を作る
- **THEN** 完了のコメントのマーカーに `sha=abc123` と `cr=48` があり、エージェントは `abc123` のCIを見張り始める

### Requirement: 見張りの対象
エージェントは、担当プロジェクトのopenで `agent:done` のissueのうち、最新の完了のコメントのマーカーに `sha` と `cr` があるものだけを見張らなければならない（MUST）。
見張りの対象は、端末に記録せず、issueのコメントのマーカーから導かなければならない（MUST）。

#### Scenario: 再起動した
- **WHEN** issue 12のCIを見張っている間にエージェントが止まり、起動し直す
- **THEN** エージェントは、完了のコメントのマーカーからissue 12を見張りの対象に戻し、`sha` のCIを問い合わせる

#### Scenario: この機能の前に完了したissue
- **WHEN** 完了のコメントのマーカーに `sha` が無い `agent:done` のissueがある
- **THEN** エージェントはそのissueのCIを問い合わせない

### Requirement: 問い合わせの間隔
見張っているissueがある間、エージェントは、`poll_interval` ごとに、そのコミットのCIの状態をフォージに問い合わせなければならない（MUST）。中継につながっている間も、同じ間隔で問い合わせる。

#### Scenario: 中継につながっている
- **WHEN** 中継につながったエージェントが、`poll_interval` が60秒の設定で、1件のissueのCIを見張っている
- **THEN** エージェントは `reconcile_interval` を待たずに、60秒ごとにCIの状態を問い合わせる

### Requirement: 見張りの終了
次のどれかのとき、エージェントは、そのissueの見張りをやめなければならない（MUST）。
CIが全部終わったとき、変更要求が閉じられたかマージされたとき、変更要求のheadが `sha` と違うコミットに変わったとき、issueが閉じられたか `agent:done` でなくなったときが該当する。

#### Scenario: 人が同じブランチにpushした
- **WHEN** 見張りの間に、人が `switchboard/issue-12` に別のコミットをpushする
- **THEN** エージェントは見張りをやめ、そのコミットのCIが失敗しても直す実行を始めない

#### Scenario: PRがマージされた
- **WHEN** CIが終わる前に、人がPRをマージする
- **THEN** エージェントは見張りをやめ、issueに何も書かない

### Requirement: CIの無いリポジトリ
pushから10分たってもそのコミットのCIが1つも現れないとき、エージェントはCIの見張りをやめなければならない（MUST）。issueには何も書かない。衝突の見張りは続ける。

#### Scenario: CIを持たないリポジトリ
- **WHEN** ワークフローとpipelineの無いリポジトリで、エージェントがPRを作って完了する
- **THEN** 10分後にCIの見張りが終わり、issueは `agent:done` のまま、コメントも増えない

#### Scenario: 衝突したPR
- **WHEN** PRがbaseと衝突していて、GitHubが `pull_request` のCIを走らせない
- **THEN** CIの見張りは10分後に終わるが、衝突の見張りが衝突を見つけて直す実行を始める

### Requirement: 見張りの時間切れ
pushから6時間たってもCIが全部終わらないとき、エージェントは見張りをやめ、監査ログに記録しなければならない（MUST）。
そのとき、エージェントはissueにコメントしてはならない（MUST NOT）。

#### Scenario: 終わらないCI
- **WHEN** jobの1つが、pushから6時間たっても実行待ちのままである
- **THEN** エージェントは見張りをやめ、監査ログに記録する。issueは `agent:done` のまま、コメントも増えない

### Requirement: 失敗の判定
エージェントは、そのコミットのCI（GitHubのcheck runとcommit status、GitLabのpipeline）が全部終わってから、成否を判定しなければならない（MUST）。
結論が `failure` か `timed_out` のものが1つでもあるとき、エージェントはCIが失敗したと判定しなければならない（MUST）。
結論が `cancelled`、`skipped`、`neutral` のものを、エージェントは失敗として数えてはならない（MUST NOT）。

#### Scenario: 一部のjobがまだ動いている
- **WHEN** 2つのjobのうち1つは `failure` で終わり、もう1つはまだ動いている
- **THEN** エージェントはまだ判定せず、見張りを続ける

#### Scenario: 取り消されたjob
- **WHEN** 全部のjobが終わり、結論は `success` と `cancelled` である
- **THEN** CIは失敗とされず、見張りが終わり、issueは `agent:done` のままである

### Requirement: 失敗の情報
CIが失敗したとき、エージェントは、失敗したチェックの名前、結論、URLをフォージから集めなければならない（MUST）。
GitHub ActionsのjobとGitLabのjobでは、エージェントは、失敗したjobを最大3つまで選び、失敗したstepの名前とログの末尾を集めなければならない（MUST）。ログの末尾は、jobごとに200行と20KBのうち短いほうまでとする。
ログを集めるとき、エージェントは、ANSIの色の符号と行頭の時刻を取り除き、トークンや秘密鍵の形式の文字列を伏せ字にしなければならない（MUST）。
ログを取れない外部のCI（commit statusだけのもの）では、エージェントは、名前、説明、URLだけを集めなければならない（MUST）。

#### Scenario: 長いログ
- **WHEN** 失敗したjobのログが5,000行ある
- **THEN** 集めるのは末尾の200行（20KBを超えるならその末尾の20KB）だけで、色の符号と行頭の時刻は無い

#### Scenario: ログにトークンがある
- **WHEN** 失敗したjobのログに `ghp_` で始まる文字列がある
- **THEN** 集めた情報では、その文字列が伏せ字になっている

#### Scenario: 外部のCI
- **WHEN** 外部のCIがcommit statusで `failure` を付ける
- **THEN** 集めた情報には、そのstatusの名前、説明、URLが入り、ログは入らない

### Requirement: 直す実行の開始
CIが失敗し、直した回数が `<agent.max_fix_attempts>` 未満のとき、エージェントは、`agent:running` に変えてから直す実行を始めなければならない（MUST）。
そのとき、エージェントは、失敗したチェックの名前と、何回目の直しかを書いた着手のコメントを投稿しなければならない（MUST）。そのマーカーの `reason` は `fix` とする。

#### Scenario: 初めてCIが失敗する
- **WHEN** `<agent.max_fix_attempts>` が2の設定で、issue 12のPRのCIのjob `test` が失敗する
- **THEN** issueは `agent:running` になり、「CIが失敗したので直します（1/2）」と `test` を書いた、`reason=fix` のマーカーのコメントが付き、直す実行が始まる

#### Scenario: 計画のモードのissue
- **WHEN** 計画の承認の後に実装して完了したissueで、CIが失敗する
- **THEN** 計画の実行ではなく、直す実行が始まる

### Requirement: 直す回数の上限に達したとき
CIが失敗し、直した回数が `<agent.max_fix_attempts>` に達しているとき、エージェントは、issueを `waiting(reason=ci)` にしなければならない（MUST）。直す実行は始めない。
そのコメントには、失敗したチェックの名前と結論とURL、自動で直した回数と各回の要約、ログの末尾40行を書かなければならない（MUST）。ログは `<details>` の中に書く。

#### Scenario: 2回直しても失敗する
- **WHEN** `<agent.max_fix_attempts>` が2の設定で、2回目の直しの後のCIも失敗する
- **THEN** 直す実行は始まらず、issueは `agent:waiting` になる。コメントには失敗したチェックと、2回の直しの要約と、折りたたんだログの末尾がある

#### Scenario: 自動では直さない設定
- **WHEN** `<agent.max_fix_attempts>` が0の設定で、CIが失敗する
- **THEN** 直す実行は始まらず、issueは `waiting(reason=ci)` になる

### Requirement: CIを読む権限が無いとき
CIの状態かログを読む問い合わせに、フォージが権限の不足（403）を返したとき、エージェントは、そのissueの見張りをやめ、監査ログに記録しなければならない（MUST）。
そのとき、エージェントはissueにコメントしてはならない（MUST NOT）。

#### Scenario: Actionsの権限が無いトークン
- **WHEN** `Actions: Read` の無いGitHubのトークンで、エージェントがCIのログを読もうとし、403が返る
- **THEN** 見張りが終わり、監査ログに記録される。issueは `agent:done` のまま、コメントも増えない

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
