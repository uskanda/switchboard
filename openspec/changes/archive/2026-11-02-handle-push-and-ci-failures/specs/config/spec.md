# Spec Delta

## ADDED Requirements

### Requirement: 自動で直す回数の設定
設定の `agent` は、整数の `<agent.max_fix_attempts>` を持てなければならない（MUST）。無いときは2とする。
`<agent.max_fix_attempts>` が0のとき、エージェントは、CIの失敗とnon-fast-forwardを自動では直さない。
`<agent.max_fix_attempts>` が負のとき、`switchboard` は設定の誤りとして拒まなければならない（MUST）。

#### Scenario: 項目が無い設定
- **WHEN** `<agent.max_fix_attempts>` の無い設定を読む
- **THEN** 誤りにならず、自動で直す回数の上限は2になる

#### Scenario: 負の値
- **WHEN** `<agent.max_fix_attempts>` が-1の設定を読む
- **THEN** 設定の誤りとして、`<agent.max_fix_attempts>` が報告される
