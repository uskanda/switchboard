## MODIFIED Requirements

### Requirement: 計画の実行の権限
計画の実行では、エージェントは、`claude -p` に読み取りだけの権限を渡さなければならない（MUST）。
許すのは、`Read`、`Grep`、`Glob`、`WebSearch` と、`git status`、`git diff`、`git log` とする。
`Edit`、`Write`、`NotebookEdit`、`git add`、`git commit`、`git push`、`make` は拒否する。
エージェントは、計画の実行に、`--tools` で `Bash`、`Read`、`Glob`、`Grep`、`WebSearch` だけを渡さなければならない（MUST）。

#### Scenario: 計画の実行がファイルを書こうとする
- **WHEN** 計画の実行の中で、Claudeが `Write` を呼ぶ
- **THEN** 呼び出しは拒否され、ファイルは書かれない

#### Scenario: 計画の実行のツール
- **WHEN** 計画の実行を始める
- **THEN** 実行のinitの `tools` に `Edit`、`Write`、`NotebookEdit` は無い
