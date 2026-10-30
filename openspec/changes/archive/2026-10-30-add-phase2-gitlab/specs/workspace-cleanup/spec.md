## Purpose

閉じられたissueのために作ったworktreeとローカルのブランチを片付け、端末にworktreeが溜まり続けないようにする。

## ADDED Requirements

### Requirement: 閉じられたissueの片付け
担当プロジェクトの閉じられたissueにworktreeがあるとき、エージェントは、そのworktreeを消さなければならない（MUST）。`switchboard/issue-<n>` のローカルのブランチも消す。
片付けは、フォージにかかわらず同じ規則で行う。
エージェントは、リモートのブランチを消してはならない（MUST NOT）。

#### Scenario: マージでissueが閉じられた
- **WHEN** `Closes #12` を持つPRがマージされ、issue 12が閉じられる
- **THEN** 次のポーリングで、issue 12のworktreeと `switchboard/issue-12` のローカルのブランチが消える。リモートのブランチは残る

### Requirement: 残っている変更の保護
worktreeに、コミットしていない変更があるとき、エージェントは、そのworktreeとブランチを消してはならない（MUST NOT）。
そのとき、エージェントは、消さなかったworktreeと理由をログに書かなければならない（MUST）。

#### Scenario: コミットしていない変更がある
- **WHEN** 閉じられたissue 12のworktreeに、コミットしていないファイルの変更がある
- **THEN** worktreeとブランチは残り、ログに、issue 12のworktreeをコミットしていない変更があるため残したと書かれる

### Requirement: 実行中のissueは片付けない
issueの実行が動いている間、エージェントは、そのissueのworktreeを消してはならない（MUST NOT）。

#### Scenario: 実行中に閉じられた
- **WHEN** issue 12の実行中に、issue 12が閉じられる
- **THEN** 実行が止まった後で片付けが行われ、実行中のworktreeは消されない
