# Spec Delta

## MODIFIED Requirements

### Requirement: 終了の宣言
実装の実行では、エージェントは、`--json-schema` で終了の宣言のスキーマを渡して `claude -p` を起動しなければならない（MUST）。
宣言は、`status`（`waiting` か `done`）と `summary` を必ず持ち、`waiting` のときは `question` も持つ。
計画の実行の終了の宣言は、`plan-mode` で決める。

#### Scenario: 質問を残して終わる
- **WHEN** Claudeが判断に迷い、`status: waiting` と質問を宣言して終わる
- **THEN** resultの `structured_output` から宣言が読み取られる

### Requirement: 待ちの書き戻し
実行が `waiting` を宣言したとき、エージェントは、質問をissueにコメントし、状態を `agent:waiting` に変えなければならない（MUST）。
実装の実行で、resultの `permission_denials` が空でないとき、エージェントは `reason` を `permission` にしなければならない（MUST）。拒否された操作はコメントに書く。
計画の実行で、resultの `permission_denials` が空でないとき、エージェントは `reason` を `permission` にしてはならない（MUST NOT）。
`reason` は `plan-questions` か `plan` のままとし、拒否された操作は、コメントの末尾の折りたたみの節に一覧で書く。

#### Scenario: 権限の拒否で止まった
- **WHEN** 実行が `waiting` を宣言し、resultの `permission_denials` にBashのコマンドが1つある
- **THEN** コメントに質問と拒否されたコマンドが書かれ、マーカーの `reason` が `permission` になる

#### Scenario: 計画の実行で権限が拒否された
- **WHEN** 計画の実行が質問を宣言して終わり、resultの `permission_denials` に `go test ./...` がある
- **THEN** マーカーの `reason` は `plan-questions` で、`go test ./...` はコメントの末尾の `<details>` の中に書かれる

### Requirement: マーカー
エージェントがissueに書くコメントは、末尾の単独の行にマーカーを持たなければならない（MUST）。
マーカーは `<!-- switchboard v=1 ... -->` の形で、`host`、`session`、`run`、`state`、`reason`、`ack`、`at` を持つ。
`ack` は、その実行がプロンプトに取り込んだ最後のコメントのIDとする。
始めるラベルで着手したときの着手のコメントのマーカーは、`mode`（`plan` か `direct`）も持つ。質問のコメント（`reason=plan-questions`）のマーカーは、そのコメントの最後の質問の番号の `q` も持つ。
マーカーを読むとき、エージェントは、知らない項目を読み飛ばさなければならない（MUST）。

#### Scenario: 待ちのマーカー
- **WHEN** issue 12の実行が質問を残して終わる
- **THEN** コメントの最後の行に、`state=waiting` と、取り込んだ最後のコメントIDを `ack` に持つマーカーがある

#### Scenario: 計画の質問のマーカー
- **WHEN** 計画の実行がQ4とQ5を聞くコメントを投稿する
- **THEN** マーカーに `reason=plan-questions` と `q=5` がある
