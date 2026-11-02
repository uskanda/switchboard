## ADDED Requirements

### Requirement: 実行中のissue
APIの一覧は、実行中のissueを、状態（`running`）とともに返さなければならない（MUST）。項目は `switchboard inbox --json` と同じにする。
APIの1件の詳細は、実行中のissueについて、進捗のコメントの全文を返さなければならない（MUST）。実行中のissueへの返答は、待ちでないissueへの返答として拒む。

#### Scenario: 実行中のissueを開く
- **WHEN** 実行中のissueの詳細を求める
- **THEN** 状態は `running` で、進捗のコメントの全文が返る
