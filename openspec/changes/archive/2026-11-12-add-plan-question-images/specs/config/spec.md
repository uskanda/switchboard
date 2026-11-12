## ADDED Requirements

### Requirement: ブラウザの設定
設定は、`<agent.browser>` に、撮影に使うブラウザの実行ファイルのパスを持てなければならない（MUST）。`~/` は展開する。
`<agent.browser>` が空ではなく、そのパスに実行できるファイルが無いとき、`switchboard` は、設定の誤りとして報告しなければならない（MUST）。
この項目が無い設定は、今までどおり読めなければならない（MUST）。

#### Scenario: 無いパス
- **WHEN** `<agent.browser>` に存在しないパスを書いた設定で `switchboard agent run` を実行する
- **THEN** コマンドは0以外の終了コードで終わり、`<agent.browser>` のパスが無いと報告する

### Requirement: 計画の試作の設定
プロジェクトの設定は、`<projects[].plan_prototype>` に、計画の実行がworktreeを一時的に書き換えて試作することを許すかどうかを持てなければならない（MUST）。既定は無効とする。

#### Scenario: 項目が無い設定
- **WHEN** `<projects[].plan_prototype>` が無いプロジェクトの計画の実行を始める
- **THEN** 計画の実行の `Edit` と `Write` は拒否される
