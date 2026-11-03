# Spec Delta

## MODIFIED Requirements

### Requirement: 直す実行の開始
CIが失敗し、直した回数が `<agent.max_fix_attempts>` 未満のとき、エージェントは、`agent:running` に変えてから直す実行を始めなければならない（MUST）。
そのとき、エージェントは、失敗したチェックの名前と、何回目の直しかを書いた着手のコメントを投稿しなければならない（MUST）。そのマーカーの `reason` は `fix` とする。
スロットが無いとき、エージェントは、その直す実行をスロットを待つものとして覚え、スロットが空いた時点で始めなければならない（MUST）。始める前に、PR/MRのhead、CIの結果、issueを読み直す。
スロットを待つ直す実行について、エージェントは、順番待ちのコメントを投稿してはならない（MUST NOT）。待ち始めたことは、ログに1回だけ残す。
スロットを待つあいだにPR/MRのheadが動いたか、CIの失敗が消えたか、issueがdoneでなくなったとき、エージェントは、その直す実行を始めてはならない（MUST NOT）。

#### Scenario: 初めてCIが失敗する
- **WHEN** `<agent.max_fix_attempts>` が2の設定で、issue 12のPRのCIのjob `test` が失敗する
- **THEN** issueは `agent:running` になり、「CIが失敗したので直します（1/2）」と `test` を書いた、`reason=fix` のマーカーのコメントが付き、直す実行が始まる

#### Scenario: 計画のモードのissue
- **WHEN** 計画の承認の後に実装して完了したissueで、CIが失敗する
- **THEN** 計画の実行ではなく、直す実行が始まる

#### Scenario: スロットが空くのを待つ
- **WHEN** `<agent.max_parallel>` が1の端末で別の実行が動いている間に、issue 12のPRのCIが失敗する。そのあと動いている実行が終わる
- **THEN** issue 12に順番待ちのコメントは付かず、実行が終わった時点で、次のポーリングを待たずに直す実行が始まる

### Requirement: 衝突を直す実行の開始
変更要求がbaseと衝突し、直した回数が `<agent.max_fix_attempts>` 未満のとき、エージェントは、衝突を直す実行を始めなければならない（MUST）。
始める前に、エージェントはissueを `agent:running` に変える。
そのとき、エージェントは、baseとの衝突を直すことと、何回目の直しかを書いた着手のコメントを投稿しなければならない（MUST）。そのマーカーの `reason` は `fix` とする。
スロットが無いとき、エージェントは、衝突を直す実行をスロットを待つものとして覚え、スロットが空いた時点で始めなければならない（MUST）。`reconcile_interval` がまだ経っていなくても待たない。始める前に、変更要求とissueを読み直す。
スロットを待つ衝突を直す実行について、エージェントは、順番待ちのコメントを投稿してはならない（MUST NOT）。待ち始めたことは、ログに1回だけ残す。
スロットを待つあいだに衝突が消えたか、変更要求のheadが動いたとき、エージェントは、衝突を直す実行を始めてはならない（MUST NOT）。

#### Scenario: doneの後にbaseが進んで衝突する
- **WHEN** `<agent.max_fix_attempts>` が2の設定で、issue 12のPRがbaseと衝突する
- **THEN** issueは `agent:running` になり、「baseとの衝突を解消します（1/2）」と書いた、`reason=fix` のマーカーのコメントが付き、直す実行が始まる

#### Scenario: 計画のモードのissue
- **WHEN** 計画の承認の後に実装して完了したissueのPRが、baseと衝突する
- **THEN** 計画の実行ではなく、衝突を直す実行が始まる

#### Scenario: reconcile_intervalの前にスロットが空く
- **WHEN** 衝突だけを見る段のissue 12のPRが衝突し、別の実行が動いていてスロットが無い。そのあと、`reconcile_interval` が経つ前に動いている実行が終わる
- **THEN** 実行が終わった時点で、issue 12の衝突を直す実行が始まる

#### Scenario: 待つあいだに衝突が消える
- **WHEN** issue 12のPRが衝突し、スロットを待っている。そのあいだに人がbaseを戻して衝突が消え、スロットが空く
- **THEN** 直す実行は始まらず、issueは `agent:done` のままである
