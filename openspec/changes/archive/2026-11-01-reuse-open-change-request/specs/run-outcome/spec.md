# Spec Delta

## MODIFIED Requirements

### Requirement: pushとPRの作成
出力検査を通ったとき、エージェントは `switchboard/issue-<n>` ブランチだけをpushし、`base_branch` へのdraftの変更要求を作らなければならない（MUST）。
変更要求は、GitHubではPR、GitLabではMRとする。
エージェントは、`base_branch` へpushしてはならない（MUST NOT）。
変更要求の本文は、issueを閉じる参照（`Closes #<n>`）と、実行の要約と、検証の結果を含まなければならない（MUST）。
`switchboard/issue-<n>` から `base_branch` への開いた変更要求が既にあるとき、エージェントは新しい変更要求を作ってはならない（MUST NOT）。
そのとき、エージェントは、pushの後に、既存の変更要求のURLをissueにコメントして `agent:done` にしなければならない（MUST）。

#### Scenario: 完了する
- **WHEN** 実行が `done` を宣言し、検証と出力検査を通る
- **THEN** ブランチがpushされ、draftのPRができ、issueにPRのURLのコメントが付いて `agent:done` になる

#### Scenario: GitLabで完了する
- **WHEN** GitLabのプロジェクトで、実行が `done` を宣言し、検証と出力検査を通る
- **THEN** ブランチがpushされ、draftのMRができ、issueにMRのURLのコメントが付いて `agent:done` になる

#### Scenario: 開いたPRがあるブランチで再実行して完了する
- **WHEN** `switchboard/issue-34` から `main` への開いたPR #48があるissue 34で、再実行が `done` を宣言し、検証と出力検査を通る
- **THEN** ブランチがpushされ、PRは新しく作られず、issueにPR #48のURLのコメントが付いて `agent:done` になる

#### Scenario: 閉じたPRだけがあるブランチで完了する
- **WHEN** `switchboard/issue-34` のPRが閉じられていて、開いたPRが無いissue 34で、実行が `done` を宣言し、検証と出力検査を通る
- **THEN** 新しいdraftのPRができ、issueにそのURLのコメントが付く

## ADDED Requirements

### Requirement: 変更要求の作成の失敗の理由
変更要求の作成が失敗したとき、エージェントは、フォージが応答で返した失敗の理由をissueのコメントに書かなければならない（MUST）。
GitHubでは、応答の `message` に加えて `errors` の各項目の内容を書く。

#### Scenario: GitHubが422を返す
- **WHEN** PRの作成に、GitHubが422を返す。`message` は `Validation Failed` で、`errors` の項目は `No commits between X and Y` である
- **THEN** issueは `waiting(reason=error)` になる。コメントには `Validation Failed` と `No commits between X and Y` が書かれる
