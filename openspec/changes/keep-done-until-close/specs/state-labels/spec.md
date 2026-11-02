## ADDED Requirements

### Requirement: 完了からの再開の付け替え
エージェントは、openの `agent:done` のissueを再開するとき、`agent:done` を外して `agent:running` を付けなければならない（MUST）。付け替えの規則は、待ちから再開するときと同じにする。
closeされたissueのラベルは、変えてはならない（MUST NOT）。

#### Scenario: 完了から実行中へ
- **WHEN** `agent:done` と `bug` が付いたopenのissueで、エージェントが信頼済みのコメントを受けて再開する
- **THEN** issueのラベルは `agent:running` と `bug` になり、実行が終わると `agent:done` か `agent:waiting` と `bug` になる
