# Spec Delta

## Purpose

エージェントが自分でpushしたコミットのCIを見張る。失敗を判定し、失敗の情報をフォージから集めて、直す実行を始めるか、人の判断を待つ。

## ADDED Requirements

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
pushから10分たってもそのコミットのCIが1つも現れないとき、エージェントは見張りをやめなければならない（MUST）。issueには何も書かない。

#### Scenario: CIを持たないリポジトリ
- **WHEN** ワークフローとpipelineの無いリポジトリで、エージェントがPRを作って完了する
- **THEN** 10分後に見張りが終わり、issueは `agent:done` のまま、コメントも増えない

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
