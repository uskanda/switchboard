## MODIFIED Requirements

### Requirement: 計画の実行の権限
計画の実行では、エージェントは、`claude -p` に読み取りだけの権限を渡さなければならない（MUST）。
許すのは、`Read`、`Grep`、`Glob` と、`git status`、`git diff`、`git log` とする。
読むだけのgitのサブコマンドも許す。対象は `ls-tree`、`ls-files`、`show`、`cat-file`、`merge-base`、`rev-parse`、`rev-list`、`blame`、`grep`、`shortlog`、`describe` である。
`Edit`、`Write`、`NotebookEdit`、`git add`、`git commit`、`git push`、`make` は拒否する。

#### Scenario: 計画の実行がファイルを書こうとする
- **WHEN** 計画の実行の中で、Claudeが `Write` を呼ぶ
- **THEN** 呼び出しは拒否され、ファイルは書かれない

#### Scenario: 計画の実行がbaseのファイルを調べる
- **WHEN** 計画の実行の中で、Claudeが `git show origin/main:README.md` を実行する
- **THEN** コマンドは許可される
