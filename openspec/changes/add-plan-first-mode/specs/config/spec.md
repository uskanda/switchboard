# Spec Delta

## ADDED Requirements

### Requirement: 計画のモードの設定
プロジェクトの設定は、真偽値の `<projects[].plan_first>` を持てなければならない（MUST）。無いときは偽とする。
設定は、`<plan.approve_word>` に承認の語を持てなければならない（MUST）。無いときは `/ok` とする。空の文字列や、空白を含む文字列は、設定の誤りとして拒む。
設定の `labels` は、`<labels.plan>` と `<labels.go>` を持てなければならない（MUST）。既定の名前は、それぞれ `agent:plan`、`agent:go` である。
これらの項目が無い設定は、今までどおり読めなければならない（MUST）。

#### Scenario: 承認の語に空白がある
- **WHEN** `<plan.approve_word>` が `ok please` の設定を読む
- **THEN** 設定の誤りとして、`<plan.approve_word>` が報告される

#### Scenario: 項目が無い設定
- **WHEN** `<projects[].plan_first>`、`<plan.approve_word>`、`<labels.plan>`、`<labels.go>` のどれも無い設定を読む
- **THEN** 誤りにならず、どのプロジェクトでも `agent:ready` ですぐに実装に入る
