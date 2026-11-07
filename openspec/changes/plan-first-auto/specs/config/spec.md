## MODIFIED Requirements

### Requirement: 計画のモードの設定
プロジェクトの設定は、`always`、`auto`、`never` のどれかの値をとる `<projects[].plan_first>` を持てなければならない（MUST）。無いときは `never` とする。
設定は、前の版の真偽値の `<projects[].plan_first>` を、`true` なら `always`、`false` なら `never` として読まなければならない（MUST）。
`<projects[].plan_first>` がそのほかの値のとき、設定の読み込みは、`plan_first` を名指しして設定を拒まなければならない（MUST）。
設定は、`<plan.approve_word>` に承認の語を持てなければならない（MUST）。無いときは `/ok` とする。空の文字列や、空白を含む文字列は、設定の誤りとして拒む。
設定の `labels` は、`<labels.plan>` と `<labels.go>` を持てなければならない（MUST）。既定の名前は、それぞれ `agent:plan`、`agent:go` である。
これらの項目が無い設定は、今までどおり読めなければならない（MUST）。

#### Scenario: 承認の語に空白がある
- **WHEN** `<plan.approve_word>` が `ok please` の設定を読む
- **THEN** 設定の誤りとして、`<plan.approve_word>` が報告される

#### Scenario: 項目が無い設定
- **WHEN** `<projects[].plan_first>`、`<plan.approve_word>`、`<labels.plan>`、`<labels.go>` のどれも無い設定を読む
- **THEN** 誤りにならず、どのプロジェクトでも `agent:ready` ですぐに実装に入る

#### Scenario: 前の版の真偽値
- **WHEN** `plan_first: true` のプロジェクトと `plan_first: false` のプロジェクトがある設定を読む
- **THEN** 誤りにならず、前者は `always`、後者は `never` として扱われる

#### Scenario: 知らない値
- **WHEN** `plan_first: sometimes` のプロジェクトがある設定を読む
- **THEN** 設定の誤りとして、`plan_first` が報告される
