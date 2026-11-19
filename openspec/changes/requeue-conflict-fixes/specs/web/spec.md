# Spec Delta

## ADDED Requirements

### Requirement: 衝突の直しの順番待ちの表示
順番待ちの理由が `conflict` のとき、WUIは、当たった上限の代わりに、衝突の直しを順番待ちに回したことを示さなければならない（MUST）。

#### Scenario: 衝突の直しで入り直したissue
- **WHEN** 衝突の直しが上限を超えて、issue 12が順番待ちに入り直す
- **THEN** 順番待ちの節のissue 12の行には、日本語なら「衝突の直しを順番待ちに回しています」、英語なら「Conflict fix requeued」と出る
