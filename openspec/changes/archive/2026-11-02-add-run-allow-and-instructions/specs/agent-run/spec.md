# Spec Delta

## ADDED Requirements

### Requirement: 追加の許可を使う実行
ファイルを書く実行では、エージェントは、`<agent.allow_tools_extra>` を許可するツールに加えなければならない（MUST）。
そのプロジェクトの `<projects[].allow_tools_extra>` も、同じように加える。
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
