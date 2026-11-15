## MODIFIED Requirements

### Requirement: 直す実行の開始
CIが失敗し、baseが赤でなく、直した回数が `<agent.max_fix_attempts>` 未満のとき、エージェントは直す実行を始めなければならない（MUST）。
始める前に、エージェントはissueを `agent:running` に変える。
そのとき、エージェントは、失敗したチェックの名前と、何回目の直しかを書いた着手のコメントを投稿しなければならない（MUST）。そのマーカーの `reason` は `fix` とする。

#### Scenario: 初めてCIが失敗する
- **WHEN** `<agent.max_fix_attempts>` が2の設定で、issue 12のPRのCIのjob `test` が失敗し、baseの先頭のCIは成功している
- **THEN** issueは `agent:running` になり、「CIが失敗したので直します（1/2）」と `test` を書いた、`reason=fix` のマーカーのコメントが付き、直す実行が始まる

#### Scenario: 計画のモードのissue
- **WHEN** 計画の承認の後に実装して完了したissueで、CIが失敗する
- **THEN** 計画の実行ではなく、直す実行が始まる

### Requirement: 衝突を直す実行の開始
変更要求がbaseと衝突し、baseが赤でなく、直した回数が `<agent.max_fix_attempts>` 未満のとき、エージェントは、衝突を直す実行を始めなければならない（MUST）。
始める前に、エージェントはissueを `agent:running` に変える。
そのとき、エージェントは、baseとの衝突を直すことと、何回目の直しかを書いた着手のコメントを投稿しなければならない（MUST）。そのマーカーの `reason` は `fix` とする。

#### Scenario: doneの後にbaseが進んで衝突する
- **WHEN** `<agent.max_fix_attempts>` が2の設定で、issue 12のPRがbaseと衝突する
- **THEN** issueは `agent:running` になり、「baseとの衝突を解消します（1/2）」と書いた、`reason=fix` のマーカーのコメントが付き、直す実行が始まる

#### Scenario: 計画のモードのissue
- **WHEN** 計画の承認の後に実装して完了したissueのPRが、baseと衝突する
- **THEN** 計画の実行ではなく、衝突を直す実行が始まる

## ADDED Requirements

### Requirement: baseが赤の判定
変更要求のCIが失敗したとき、エージェントは、変更要求のbaseのブランチの先頭のコミットのCIを読まなければならない（MUST）。
baseの先頭のCIが全部終わり、変更要求で失敗したチェックがどれも同じ名前でbaseの先頭でも失敗しているとき、エージェントは、baseが赤だと判定しなければならない（MUST）。失敗の数え方は「失敗の判定」に従う。
baseの先頭のCIが終わっていないとき、エージェントは、判定を次の問い合わせに回さなければならない（MUST）。そのとき、直す実行を始めてはならない（MUST NOT）。

#### Scenario: baseと同じテストで落ちる
- **WHEN** PRのCIでjob `verify` が失敗し、mainの先頭のコミットのCIでもjob `verify` が失敗している
- **THEN** エージェントはbaseが赤だと判定し、直す実行を始めない

#### Scenario: PRだけが別のjobで落ちる
- **WHEN** PRのCIでjob `verify` と `lint` が失敗し、mainの先頭のコミットのCIでは `verify` だけが失敗している
- **THEN** baseが赤とは判定されず、今までどおり直す実行が始まる

#### Scenario: baseのCIが動いている
- **WHEN** PRのCIが失敗し、mainの先頭のコミットのCIがまだ動いている
- **THEN** エージェントは直す実行を始めず、次の問い合わせでbaseをもう一度読む

#### Scenario: baseにCIが無い
- **WHEN** PRのCIが失敗し、mainの先頭のコミットにCIが1つも無い
- **THEN** baseが赤とは判定されず、今までどおり直す実行が始まる

### Requirement: baseが赤の間の待ち
baseが赤だと判定したとき、エージェントは、直す実行を始めてはならず、直した回数に数えてはならない（MUST NOT）。
そのとき、エージェントは、issueを `agent:done` のままにし、見張りを続けなければならない（MUST）。

#### Scenario: 上限の手前で待つ
- **WHEN** `<agent.max_fix_attempts>` が2の設定で、1回直したissueのPRのCIが、baseと同じjobで失敗する
- **THEN** issueは `agent:done` のままで、直した回数は1のままである。`waiting(reason=ci)` にはならない

#### Scenario: エージェントを起動し直す
- **WHEN** baseが赤で待っている間に、エージェントが止まり、起動し直す
- **THEN** エージェントは完了のコメントのマーカーから見張りを戻し、CIの失敗をもう一度baseと突き合わせて、待ちを続ける

### Requirement: 待ち始めのコメント
baseが赤で待ち始めたとき、エージェントは、baseのCIが赤なので緑に戻るまで待つことを書いたコメントを、issueに1回だけ投稿しなければならない（MUST）。
そのコメントのマーカーの `state` は `note`、`reason` は `base` としなければならない（MUST）。このマーカーは、issueの状態の読み取りと受信箱の表示を変えてはならない（MUST NOT）。
同じ見張りの間、エージェントは、待ち始めのコメントを2回以上投稿してはならない（MUST NOT）。

#### Scenario: 待ち始める
- **WHEN** issue 12のPRのCIが、baseと同じjob `verify` で失敗する
- **THEN** issue 12に、「baseのCIが失敗しているため、緑に戻るまで待ちます」と `verify` を書いたコメントが1つ付く。そのマーカーは `state=note reason=base` で、issueは受信箱で完了のまま出る

#### Scenario: 待ちが続く
- **WHEN** 待ち始めてから、`poll_interval` ごとの問い合わせが10回あり、baseはまだ赤である
- **THEN** issueのコメントは増えない

#### Scenario: エージェントと信頼するユーザーが同じアカウント
- **WHEN** エージェントと `trusted_authors` が同じアカウントで、待ち始めのコメントが投稿される
- **THEN** エージェントはそのコメントを人の返信とみなさず、issueを再開しない

### Requirement: baseが赤の間の衝突
baseが赤の間、変更要求がbaseと衝突したとき、エージェントは、衝突を直す実行を始めてはならない（MUST NOT）。直した回数にも数えない。
baseが緑に戻ったとき、衝突がまだあれば、エージェントは衝突を直す実行を始めなければならない（MUST）。

#### Scenario: 赤い間に衝突する
- **WHEN** mainの先頭のコミットのCIが失敗している間に、issue 12のPRがmainと衝突する
- **THEN** 衝突を直す実行は始まらず、issueは `agent:done` のままである

#### Scenario: 緑に戻ってから直す
- **WHEN** mainの先頭のCIが成功に戻り、issue 12のPRはまだ衝突している
- **THEN** 衝突を直す実行が始まり、「baseとの衝突を解消します（1/2）」のコメントが付く

### Requirement: baseの読み取りの間隔
baseが赤で待っているissueがある間、エージェントは、`poll_interval` ごとに、baseの先頭のコミットとそのCIを読まなければならない（MUST）。読むのは、プロジェクトとbaseのブランチの組ごとに1回だけとする。
同じ組で待つissueが何件あっても、エージェントは、1回の問い合わせで、その組のbaseを2回以上読んではならない（MUST NOT）。
待つ期間に上限は無い。変更要求がopenで、issueが `agent:done` の間、エージェントは待ちを続けなければならない（MUST）。

#### Scenario: 同じプロジェクトで10件が待つ
- **WHEN** smarternewsで10件のissueが、mainが赤のために待っている
- **THEN** 1回の問い合わせで、mainの先頭のコミットとそのCIの読み取りは1回ずつである

#### Scenario: 1日たっても赤い
- **WHEN** mainが24時間赤のままである
- **THEN** issueは `agent:done` のままで、エージェントは見張りを続ける

#### Scenario: 待っている間にPRがマージされる
- **WHEN** baseが赤で待っている間に、人がPRをマージする
- **THEN** エージェントは見張りをやめ、issueに何も書かない

### Requirement: baseが緑に戻ったときの取り込み
baseが赤で待っている間に、baseの先頭のコミットのCIが全部終わって失敗が無くなったとき、エージェントは、issueのブランチへbaseを取り込まなければならない（MUST）。取り込みにClaudeは起動しない。
取り込みが衝突しなかったとき、エージェントは、取り込んだコミットをpushし、`fix=base` と新しい `sha` を持つdoneの投稿を出し直さなければならない（MUST）。そのあと、新しいコミットのCIを見張る。
取り込みが衝突したとき、エージェントは、取り込みをやめてworktreeを元に戻し、衝突を直す実行を始めなければならない（MUST）。この衝突を直す実行は、直した回数に数える。
この取り込みは、直した回数に数えてはならない（MUST NOT）。issueを `agent:running` に変えてはならず、着手のコメントを投稿してはならない（MUST NOT）。

#### Scenario: mainが直ったあと取り込む
- **WHEN** issue 12がmainの赤で待っている間に、mainに修正のコミットが入り、そのCIが成功する
- **THEN** エージェントはissue 12のブランチにmainをmergeしてpushし、`fix=base` のdoneの投稿を出す。新しいコミットのCIの見張りが始まり、直した回数は変わらない

#### Scenario: 取り込みで衝突する
- **WHEN** mainが緑に戻り、issue 12のブランチにmainをmergeすると衝突する
- **THEN** エージェントはmergeをやめ、衝突を直す実行を始める。着手のコメントは「baseとの衝突を解消します（1/2）」である

#### Scenario: 取り込んだあとも落ちる
- **WHEN** 取り込んだコミットのCIで、mainでは成功しているjob `verify` が失敗する
- **THEN** baseが赤とは判定されず、今までどおり直す実行が始まる

#### Scenario: 自動マージの依頼がある
- **WHEN** 受信箱で自動マージを待つPRのissueで、エージェントが `fix=base` のdoneを投稿し、そのコミットのCIが通る
- **THEN** 受信箱のサービスは、そのdoneの `sha` でPRをマージする
