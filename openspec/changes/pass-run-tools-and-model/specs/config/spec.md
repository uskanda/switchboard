## ADDED Requirements

### Requirement: 実行のモデルとeffortの設定
設定の `agent` は、文字列の `<agent.model>` と `<agent.effort>` を持てなければならない（MUST）。
`agent` は、`<agent.plan>` と `<agent.fix>` に、それぞれ `model` と `effort` を持てなければならない（MUST）。
これらの項目が無い設定は、今までどおり読めなければならない（MUST）。
モデルとeffortの変化は、`projects` 以外の変化として扱い、再起動まで反映しない。

#### Scenario: 項目が無い設定
- **WHEN** `agent` に `model`、`effort`、`plan`、`fix` の無い設定を読む
- **THEN** 誤りにならず、どの実行のモデルとeffortも空になる

#### Scenario: 計画の実行だけの上書き
- **WHEN** `agent` に `model: opus` と `plan: {model: sonnet}` のある設定を読む
- **THEN** 計画の実行のモデルは `sonnet`、それ以外の実行のモデルは `opus` になる

### Requirement: effortの値の検査
設定を読むとき、`switchboard` は、`<agent.effort>`、`<agent.plan>` のeffort、`<agent.fix>` のeffortを検査しなければならない（MUST）。
空でない値が `low`、`medium`、`high`、`xhigh`、`max` のどれでもないとき、設定の誤りとして、何も変更せずに終了し、誤りのある項目を報告する。
`switchboard` は、モデルの値を検査してはならない（MUST NOT）。

#### Scenario: 知らないeffort
- **WHEN** `agent` に `effort: ultra` のある設定で `switchboard agent run` を実行する
- **THEN** コマンドは0以外の終了コードで終わり、`<agent.effort>` の `ultra` が許されない値だと報告する

#### Scenario: 知らないモデル名
- **WHEN** `agent` に `model: my-model` のある設定を読む
- **THEN** 誤りにならない
