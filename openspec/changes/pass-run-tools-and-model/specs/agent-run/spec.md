## ADDED Requirements

### Requirement: 実行に渡すツール
エージェントは、`--tools` で渡すツールを明示して `claude -p` を起動しなければならない（MUST）。
ファイルを書く実行には、`Bash`、`Read`、`Edit`、`Write`、`Glob`、`Grep`、`WebSearch` を渡す。
エージェントは、渡すツールに無いツールを、実行に使わせてはならない（MUST NOT）。
エージェントは、どの実行でも `WebSearch` を許可しなければならない（MUST）。

#### Scenario: 実装の実行のツール
- **WHEN** 追加の許可が無い設定で、実装の実行を始める
- **THEN** 実行のinitの `tools` は、`Bash`、`Read`、`Edit`、`Write`、`Glob`、`Grep`、`WebSearch` と構造化出力のツールだけである

#### Scenario: Webを検索する
- **WHEN** 実装の実行の中で、Claudeが `WebSearch` を呼ぶ
- **THEN** 呼び出しは許可され、resultの `permission_denials` に記録されない

### Requirement: 実行のモデルとeffort
エージェントは、設定で決まったモデルを `--model` で、effortを `--effort` で渡して `claude -p` を起動しなければならない（MUST）。
計画の実行は `<agent.plan>` の値を、直す実行は `<agent.fix>` の値を使う。その値が空のときは、`<agent.model>` と `<agent.effort>` を使う。
値が決まらないとき、エージェントはそのフラグを付けてはならない（MUST NOT）。

#### Scenario: 既定のモデル
- **WHEN** `<agent.model>` が `opus` で、`<agent.plan>` にモデルが無い設定で、計画の実行を始める
- **THEN** `claude -p` は `--model opus` で起動する

#### Scenario: 直す実行の上書き
- **WHEN** `<agent.effort>` が `high` で、`<agent.fix>` のeffortが `medium` の設定で、CIの失敗を直す実行を始める
- **THEN** `claude -p` は `--effort medium` で起動する

#### Scenario: 設定が無い
- **WHEN** モデルとeffortの項目が無い設定で、実装の実行を始める
- **THEN** `claude -p` は `--model` と `--effort` を付けずに起動する

## MODIFIED Requirements

### Requirement: 追加の許可を使う実行
ファイルを書く実行では、エージェントは、`<agent.allow_tools_extra>` を許可するツールに加えなければならない（MUST）。
そのプロジェクトの `<projects[].allow_tools_extra>` も、同じように加える。
エージェントは、加えた各項目のツール名（`Bash(openspec *)` なら `Bash`）を、渡すツールにも加えなければならない（MUST）。
ファイルを書く実行とは、直接の実装、承認のあとの実装、CIとpushの失敗を直す実行、baseとの衝突を直す実行である。
エージェントは、計画の実行に、追加の許可を加えてはならない（MUST NOT）。
追加の許可があっても、組み込みで拒否している操作は、拒否したままでなければならない（MUST）。

#### Scenario: スキルを使う
- **WHEN** `<agent.allow_tools_extra>` に `Skill` がある状態で、承認のあとの実装の実行の中で、ClaudeがSkillツールを呼ぶ
- **THEN** 実行のinitの `tools` に `Skill` があり、呼び出しは許可され、resultの `permission_denials` に記録されない

#### Scenario: プロジェクトだけの許可
- **WHEN** `Bash(openspec *)` は `gh:uskanda/switchboard` だけにある。別のプロジェクトの実装の実行で、Claudeが `openspec list` を実行しようとする
- **THEN** コマンドは拒否され、resultの `permission_denials` に記録される

#### Scenario: 計画の実行
- **WHEN** `<agent.allow_tools_extra>` に `Skill` がある状態で、計画の実行を始める
- **THEN** 実行のinitの `tools` に `Skill` は無い
