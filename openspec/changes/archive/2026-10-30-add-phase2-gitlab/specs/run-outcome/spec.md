## MODIFIED Requirements

### Requirement: push前の出力検査
pushの前に、エージェントは次の4点を確かめなければならない（MUST）。
1点でも満たさないとき、エージェントはpushせず、issueを `waiting(reason=verify)` にしなければならない（MUST）。

- ブランチが `switchboard/issue-<n>` で、`base_branch` より先にコミットがある
- 差分が保護パスに触れていない
- 差分、コミットメッセージ、投稿する文章に、トークンや秘密鍵の形式の文字列が無い
- 完了の検証を通っている

保護パスの既定は、`.github/workflows/`、`.gitlab-ci.yml`、`.claude/`、`.mcp.json`、`CLAUDE.md`、`AGENTS.md` とする。
`protected_paths_extra` に書いたパスも、保護パスに加える。
プロジェクトのフォージにかかわらず、同じ保護パスを使う。

#### Scenario: ワークフローの変更
- **WHEN** 実行が `done` を宣言し、差分が `.github/workflows/ci.yml` を変えている
- **THEN** pushされず、issueは待ちになり、保護パスに触れたことがコメントに書かれる

#### Scenario: GitLabのCI定義の変更
- **WHEN** GitLabのプロジェクトで、実行が `done` を宣言し、差分が `.gitlab-ci.yml` を変えている
- **THEN** pushされず、issueは待ちになり、保護パスに触れたことがコメントに書かれる

#### Scenario: 秘密情報の混入
- **WHEN** 差分に `ghp_` で始まる文字列がある
- **THEN** pushされず、issueは待ちになる。コメントには該当するファイルの名前が書かれ、文字列そのものは書かれない

### Requirement: pushとPRの作成
出力検査を通ったとき、エージェントは `switchboard/issue-<n>` ブランチだけをpushし、`base_branch` へのdraftの変更要求を作らなければならない（MUST）。
変更要求は、GitHubではPR、GitLabではMRとする。
エージェントは、`base_branch` へpushしてはならない（MUST NOT）。
変更要求の本文は、issueを閉じる参照（`Closes #<n>`）と、実行の要約と、検証の結果を含まなければならない（MUST）。

#### Scenario: 完了する
- **WHEN** 実行が `done` を宣言し、検証と出力検査を通る
- **THEN** ブランチがpushされ、draftのPRができ、issueにPRのURLのコメントが付いて `agent:done` になる

#### Scenario: GitLabで完了する
- **WHEN** GitLabのプロジェクトで、実行が `done` を宣言し、検証と出力検査を通る
- **THEN** ブランチがpushされ、draftのMRができ、issueにMRのURLのコメントが付いて `agent:done` になる
