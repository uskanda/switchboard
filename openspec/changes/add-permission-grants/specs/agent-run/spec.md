## MODIFIED Requirements

### Requirement: 追加の許可を使う実行
ファイルを書く実行では、エージェントは、`<agent.allow_tools_extra>` を許可するツールに加えなければならない（MUST）。
そのプロジェクトの `<projects[].allow_tools_extra>` も、同じように加える。
そのあとに、`permission-grants` の許可のファイルの規則を、全体、プロジェクト、issueの順に加える。
ファイルを書く実行とは、直接の実装、承認のあとの実装、CIとpushの失敗を直す実行、baseとの衝突を直す実行である。
エージェントは、計画の実行に、追加の許可を加えてはならない（MUST NOT）。
追加の許可があっても、組み込みで拒否している操作は、拒否したままでなければならない（MUST）。

#### Scenario: スキルを使う
- **WHEN** `<agent.allow_tools_extra>` に `Skill` がある状態で、承認のあとの実装の実行の中で、ClaudeがSkillツールを呼ぶ
- **THEN** 呼び出しは許可され、resultの `permission_denials` に記録されない

#### Scenario: プロジェクトだけの許可
- **WHEN** `Bash(openspec *)` は `gh:uskanda/switchboard` だけにある。別のプロジェクトの実装の実行で、Claudeが `openspec list` を実行しようとする
- **THEN** コマンドは拒否され、resultの `permission_denials` に記録される

#### Scenario: 計画の実行
- **WHEN** `<agent.allow_tools_extra>` に `Skill` がある状態で、計画の実行の中で、ClaudeがSkillツールを呼ぼうとする
- **THEN** 呼び出しは拒否され、resultの `permission_denials` に記録される

#### Scenario: issueの許可
- **WHEN** issue 217の許可のファイルに `Bash(openspec *)` がある状態で、issue 217の直す実行の中で、Claudeが `openspec list` を実行する
- **THEN** コマンドは許可され、resultの `permission_denials` に記録されない

## ADDED Requirements

### Requirement: ファイルを書く実行で既定で許すgitの操作
ファイルを書く実行では、エージェントは、次のgitのサブコマンドを許可しなければならない（MUST）。引数の有無は問わない。
- 今までのもの：`status`、`diff`、`log`、`add`、`commit`、`merge`
- 読むだけのもの：`ls-tree`、`ls-files`、`show`、`cat-file`、`merge-base`、`rev-parse`、`rev-list`、`blame`、`grep`、`shortlog`、`describe`
- worktreeの中だけを変えるもの：`mv`、`rm`、`restore`、`stash`
エージェントは、既定では `fetch`、`checkout`、`switch`、`reset`、`branch`、`config` と、`git -c` を許してはならない（MUST NOT）。

#### Scenario: マイグレーションの番号を付け替える
- **WHEN** 実装の実行で、Claudeが `git mv migrations/033_a.sql migrations/037_a.sql` を実行する
- **THEN** コマンドは許可され、resultの `permission_denials` に記録されない

#### Scenario: baseのファイルを調べる
- **WHEN** 直す実行で、Claudeが `git ls-tree --name-only origin/main migrations/` を実行する
- **THEN** コマンドは許可される

#### Scenario: fetchする
- **WHEN** 実装の実行で、Claudeが `git fetch origin main` を実行しようとする
- **THEN** コマンドは拒否され、resultの `permission_denials` に記録される

### Requirement: fetch済みの案内
エージェントは、すべての実行のsystem promptに、`origin/<base>` は実行の前にfetch済みで、`git fetch` は許されないことを書かなければならない（MUST）。

#### Scenario: mainの最新を確かめたい
- **WHEN** 直す実行が始まる
- **THEN** system promptに、`origin/main` がfetch済みであることと、`git fetch` が許されないことが書かれている
