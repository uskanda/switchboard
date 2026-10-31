# Spec Delta

## MODIFIED Requirements

### Requirement: 着手の許可
始めるラベルが付いたissueについて、そのラベルを最後に付けたユーザーが信頼済みでないとき、エージェントは実行を始めてはならない（MUST NOT）。
始めるラベルは、`agent:ready`、`agent:plan`、`agent:go` の3つとする。

#### Scenario: 第三者がラベルを付けた
- **WHEN** 信頼済みでないユーザーが、issueに `agent:ready` を付ける
- **THEN** 実行は始まらず、判定と理由が監査ログに記録される。issueにはコメントもラベルの変更もされない

#### Scenario: 第三者がagent:goを付けた
- **WHEN** 信頼済みでないユーザーが、issueに `agent:go` を付ける
- **THEN** 実行は始まらず、判定と理由が監査ログに記録される。issueにはコメントもラベルの変更もされない
