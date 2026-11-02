## MODIFIED Requirements

### Requirement: 設定の読み直し
`switchboard agent run` と `switchboard inbox serve` は、動いている間、10秒ごとに設定ファイルの更新の時刻と中身を確かめなければならない（MUST）。
中身が変わり、検証を通ったとき、`switchboard` は `projects` を新しい設定に入れ替えなければならない（MUST）。再起動は要らない。
中身が変わり、検証を通ったとき、`switchboard agent run` は `<agent.max_parallel>` と `<agent.prefer_plan>` も新しい値に入れ替えなければならない（MUST）。

#### Scenario: 手でプロジェクトを足した
- **WHEN** `switchboard agent run` が動いている間に、`config.yaml` の `projects` に手で1項目を足して保存する
- **THEN** 10秒ほどのうちに、エージェントはそのプロジェクトを扱い始める

#### Scenario: 全体の上限を変えた
- **WHEN** `switchboard agent run` が動いている間に、`<agent.max_parallel>` を1から2に変えて保存する
- **THEN** 10秒ほどのうちに、エージェントは2つの実行を同時に動かせるようになり、再起動を求めるログは残らない

### Requirement: projects以外の変化
読み直した設定で、`projects`、`<agent.max_parallel>`、`<agent.prefer_plan>` 以外の項目が変わっているとき、`switchboard` はその項目を反映してはならない（MUST NOT）。再起動するまで反映しないことを、変わった節の名前とともにログに残す。

#### Scenario: 間隔を変えた
- **WHEN** `switchboard agent run` の動作中に、`<agent.poll_interval>` を変えて保存する
- **THEN** ポーリングの間隔は変わらない。`agent` の変化は再起動まで反映しないと、ログに残る


## ADDED Requirements

### Requirement: 計画を先にする設定
設定の `agent` は、真偽値の `<agent.prefer_plan>` を持てなければならない（MUST）。無いときはfalseとする。

#### Scenario: 設定が無い
- **WHEN** `agent` に `prefer_plan` の無い設定を読む
- **THEN** `<agent.prefer_plan>` はfalseになる

#### Scenario: 設定がある
- **WHEN** `agent` に `prefer_plan: true` のある設定を読む
- **THEN** 誤りにならず、`<agent.prefer_plan>` はtrueになる

#### Scenario: 動作中に計画の優先をオンにした
- **WHEN** `switchboard agent run` が動いている間に、`<agent.prefer_plan>` をfalseからtrueに変えて保存する
- **THEN** 10秒ほどのうちに新しい値が反映され、再起動を求めるログは残らない
