## MODIFIED Requirements

### Requirement: 直す実行で許すgitの操作
non-fast-forwardを直す実行と、変更要求の衝突を直す実行では、エージェントは、`git merge` を許可するツールに加えなければならない（MUST）。
エージェントは、どの実行でも、`git rebase` と `git push` を許してはならない（MUST NOT）。

#### Scenario: リモートの枝を取り込む
- **WHEN** non-fast-forwardを直す実行で、Claudeが `git merge origin/switchboard/issue-12` を実行する
- **THEN** コマンドは許可され、確認を求められずに実行される

#### Scenario: baseを取り込む
- **WHEN** 衝突を直す実行で、Claudeが `git merge origin/main` を実行する
- **THEN** コマンドは許可され、確認を求められずに実行される

#### Scenario: 履歴を書き換えようとする
- **WHEN** 直す実行で、Claudeが `git rebase origin/switchboard/issue-12` を実行しようとする
- **THEN** コマンドは拒否され、resultの `permission_denials` に記録される

## ADDED Requirements

### Requirement: 衝突を直す実行の指示
衝突を直す実行では、エージェントは、baseの最新を取り込んでから、`git merge origin/<base>` で衝突を解消するようClaudeに指示しなければならない（MUST）。
両方の変更が矛盾して解消できないとき、Claudeは `waiting` で質問して終わる。

#### Scenario: 解消できる衝突
- **WHEN** issue 12のPRがbaseと衝突し、衝突を直す実行が始まる
- **THEN** プロンプトはbaseとの衝突を解消する作業を指示し、Claudeはmergeのコミットを作って `done` を宣言する

#### Scenario: 矛盾する変更
- **WHEN** baseで消された関数を、issue 12の変更が使っていて、どちらを残すか決められない
- **THEN** Claudeは `waiting` で質問し、issueは `agent:waiting` になる
