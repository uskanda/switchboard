# Spec Delta

## ADDED Requirements

### Requirement: 実行の開始の記録
claudeを起動して実行を始めるとき、エージェントは、監査ログに `run.started` を1行記録しなければならない（MUST）。
その行には、issue、実行のID、実行の種類（`direct`、`plan`、`approved`、`fix`）、足した許可の一覧、全体とプロジェクトの指示のファイルごとのSHA-256を入れる。
実行の種類が `fix` のとき、その行には、直す原因（`ci`、`push`、`conflict`）を `run.fix` と同じ値で入れる。
足した許可の一覧は、実際にallowへ足した順に並べ、前後の空白を除き、既定のallowや前の項目と重なるものを省いたものとする。
指示のハッシュは、ファイルの中身そのもののSHA-256とし、保存のときに `instructions.saved` へ記録する値と同じにする。
ファイルが無いか、空白だけか、読めないとき、そのハッシュは空の文字列とする。

#### Scenario: 直接の実装
- **WHEN** 全体とプロジェクトの `allow_tools_extra` と、全体とプロジェクトの `run.md` がある状態で、直接の実装の実行を始める
- **THEN** 監査ログに `kind` が `direct` の `run.started` が1行ある。`allow` は全体の分のあとにプロジェクトの分を足した一覧である。`instructions` の値は、保存した `instructions.saved` の `sha256` と一致する

#### Scenario: pushを拒まれたあとの直す実行
- **WHEN** pushを拒まれ、直す実行が始まる
- **THEN** `run.fix` のあとに、新しい実行のIDで、`kind` が `fix`、`cause` が `push` の `run.started` が記録される

### Requirement: 計画の実行の開始の記録
計画の実行を始めるとき、エージェントは、`run.started` の足した許可を空の一覧にしなければならない（MUST）。
その行の指示のハッシュは、計画用の指示（`plan.md`）のものにする。

#### Scenario: 計画の実行
- **WHEN** 追加の許可と計画用の指示がある状態で、計画の実行を始める
- **THEN** `run.started` の `kind` は `plan`、`allow` は `[]` で、`instructions` の値は `plan.md` のハッシュである

### Requirement: 実行の開始の記録に指示の内容を書かない
エージェントは、指示の内容そのものを `run.started` に書いてはならない（MUST NOT）。

#### Scenario: 指示の中身
- **WHEN** 実装用の指示がある状態で、実行を始める
- **THEN** 監査ログのどの行にも、指示の文章が現れない
