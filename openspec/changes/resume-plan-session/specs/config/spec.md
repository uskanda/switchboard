## ADDED Requirements

### Requirement: セッションの引き継ぎの間隔の設定
設定の `agent` は、時間の長さの `<agent.resume_within>` を持てなければならない（MUST）。無いときは1時間とする。
`<agent.resume_within>` が0のとき、エージェントは、計画の実行で前回のセッションを引き継がない。
`<agent.resume_within>` が負のとき、`switchboard` は設定の誤りとして拒まなければならない（MUST）。
`<agent.resume_within>` の変化は、`projects` 以外の変化として扱い、再起動まで反映しない。

#### Scenario: 項目が無い設定
- **WHEN** `<agent.resume_within>` の無い設定を読む
- **THEN** 誤りにならず、引き継ぎの間隔は1時間になる

#### Scenario: 0の設定
- **WHEN** `<agent.resume_within>` が0の設定で、前回の計画の実行の5分後に次の計画の実行を始める
- **THEN** `--resume` を付けずに起動する

#### Scenario: 負の値
- **WHEN** `<agent.resume_within>` が `-1m` の設定を読む
- **THEN** 設定の誤りとして、`<agent.resume_within>` が報告される
