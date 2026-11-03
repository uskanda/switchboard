## MODIFIED Requirements

### Requirement: マーカー
エージェントがissueに書くコメントは、末尾の単独の行にマーカーを持たなければならない（MUST）。
マーカーは `<!-- switchboard v=1 ... -->` の形で、`host`、`session`、`run`、`state`、`reason`、`ack`、`at` を持つ。
`ack` は、その実行がプロンプトに取り込んだ最後のコメントのIDとする。
始めるラベルで着手したときの着手のコメントのマーカーは、`mode`（`plan` か `direct`）も持つ。質問のコメント（`reason=plan-questions`）のマーカーは、そのコメントの最後の質問の番号の `q` も持つ。
変更要求のURLを書いた完了のコメントのマーカーは、pushしたコミットのSHAの `sha` と、変更要求の番号の `cr` も持つ。
直す実行の着手のコメントのマーカーは、`reason=fix` を持つ。
直す実行が完了で終えたときの完了のコメントのマーカーは、直したものの種類を `fix` に持たなければならない（MUST）。種類は、CIの失敗が `ci`、non-fast-forwardのpushが `push`、baseとの衝突が `conflict` である。直す実行でない実行の完了のマーカーは、`fix` を持ってはならない（MUST NOT）。
マーカーを読むとき、エージェントは、知らない項目を読み飛ばさなければならない（MUST）。`fix` の無いマーカーは、直す実行の投稿でないものとして読む。

#### Scenario: 待ちのマーカー
- **WHEN** issue 12の実行が質問を残して終わる
- **THEN** コメントの最後の行に、`state=waiting` と、取り込んだ最後のコメントIDを `ack` に持つマーカーがある

#### Scenario: 計画の質問のマーカー
- **WHEN** 計画の実行がQ4とQ5を聞くコメントを投稿する
- **THEN** マーカーに `reason=plan-questions` と `q=5` がある

#### Scenario: 完了のマーカー
- **WHEN** issue 12の実行が、コミット `abc123` をpushしてPR #48を作って完了する
- **THEN** 完了のコメントのマーカーに `state=done`、`sha=abc123`、`cr=48` があり、`fix` は無い

#### Scenario: 衝突を直した実行の完了のマーカー
- **WHEN** PR #48の衝突を直す実行が、コミット `def456` をpushして完了する
- **THEN** 完了のコメントのマーカーに `state=done`、`sha=def456`、`cr=48`、`fix=conflict` がある
