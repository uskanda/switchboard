## MODIFIED Requirements

### Requirement: 計画の実行の開始の記録
計画の実行を始めるとき、エージェントは、`run.started` の足した許可を空の一覧にしなければならない（MUST）。
その行の指示のハッシュは、計画用の指示（`plan.md`）のものにする。
前回のセッションを引き継いで起動するとき、エージェントは、その行の `resumed_from` に、引き継いだ元のセッションIDを入れなければならない（MUST）。引き継がないときは、`resumed_from` を空の文字列にする。
引き継ぎに失敗して起動し直したとき、エージェントは、`resumed_from` を空にした `run.started` を、同じ実行のIDでもう1行記録しなければならない（MUST）。

#### Scenario: 計画の実行
- **WHEN** 追加の許可と計画用の指示がある状態で、計画の実行を始める
- **THEN** `run.started` の `kind` は `plan`、`allow` は `[]` で、`instructions` の値は `plan.md` のハッシュである

#### Scenario: 引き継いだ計画の実行
- **WHEN** 前回のセッションを引き継ぐ条件を満たす計画の実行を始める
- **THEN** `run.started` の `resumed_from` は、前回の計画の実行の投稿のマーカーのセッションIDである
