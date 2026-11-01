## ADDED Requirements

### Requirement: 順番待ちの詳細
順番待ちのissueの詳細を求められたとき、APIは、状態を順番待ちとして、順番待ちの開始時刻と当たった上限を返さなければならない（MUST）。
順番待ちのissueに `agent:waiting` が付いているとき、APIは、待ちであることと、エージェントの最後の待ちの投稿を返さなければならない（MUST）。返答も、今までどおり受け付ける。
`agent:waiting` が付いていない順番待ちのissueについて、APIは、順番待ちのコメントを最後の投稿として返さなければならない（MUST）。

#### Scenario: 始めるラベルで順番待ちのissueを開く
- **WHEN** エージェントの投稿が順番待ちのコメントだけの、`agent:ready` のissueの詳細を求める
- **THEN** 状態は順番待ちで、最後の投稿は順番待ちのコメントであり、待ちではないと返る

#### Scenario: 返答の待ちで順番待ちのissueに返答する
- **WHEN** `agent:waiting` が付いた順番待ちのissueに、返答を送る
- **THEN** 返答は投稿され、順番が来た実行のプロンプトに入る
