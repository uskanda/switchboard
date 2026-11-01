# Proposal

## Why

PRを作った後のissueに返信して再実行すると、エージェントはpushの後にPRをもう一度作ろうとする。GitHubは同じブランチのPRが既に開いていると422を返すので、実行は毎回「PRを作れませんでした」で待ちになり、返信しても抜け出せない。issue #34はPR #48があるために、この状態で止まった。
加えて、GitHubの422の理由は応答の `errors` に入っているのに、エージェントは `message`（`Validation Failed`）しか読まない。コメントを見ても原因が分からなかった。

## What Changes

- pushの後、`switchboard/issue-<n>` から `base_branch` への開いた変更要求が既にあれば、エージェントは新しく作らず、その変更要求のURLで完了にする。pushで変更要求には新しいコミットが載る
- 開いた変更要求が無いときは、今までどおりdraftで作る
- GitHubのAPIが失敗したとき、エラー文に応答の `errors` の内容も含める。GitLabは今もエラー本文の詳細を読んでいるので変えない

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

- `run-outcome`：「pushとPRの作成」で、開いた変更要求があれば再利用する。変更要求の作成に失敗したとき、フォージが返した理由をコメントに書く

## Impact

- `internal/forge`：`Forge`に、ブランチの開いた変更要求を探す操作を足す。GitHub、GitLab、fakeの実装
- `internal/forge/github`：エラーの応答から `errors` を読む
- `internal/agent/run.go`：`finish` で、作る前に開いた変更要求を探す
- 待ちで止まっているissue（#34）は、修正を入れたエージェントで返信すれば、既存のPR #48のURLで完了になる
