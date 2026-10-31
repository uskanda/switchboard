## ADDED Requirements

### Requirement: 実行中の一覧
`switchboard inbox` は、待ちの一覧の下に、`agent:running` が付いたopenのissueを、別の欄として表示しなければならない（MUST）。対象のプロジェクトとフォージの絞り込みは、待ちの一覧と同じにする。
実行中の各行は、プロジェクト、issue番号、端末の名前、経過時間、最新の進捗の1行を持つ。端末の名前と着手した時刻は、エージェントの着手のコメントのマーカーから、最新の進捗の1行は進捗のコメントから取る。進捗のコメントが無ければ `?` とする。
実行中の行には行番号を付けず、`switchboard reply` と `switchboard show` の行番号の対象にしない。
`--json` が付いたとき、各要素は状態（`waiting` か `running`）を持たなければならない（MUST）。

#### Scenario: 待ちと実行中
- **WHEN** 待ちのissueが1件、実行中のissueが1件ある状態で `switchboard inbox` を実行する
- **THEN** 待ちの欄に1行、実行中の欄に1行が出て、実行中の行には最新の進捗の1行が出る

#### Scenario: JSONの状態
- **WHEN** 同じ状態で `switchboard inbox --json` を実行する
- **THEN** 2要素の配列が出て、それぞれの状態は `waiting` と `running` である
