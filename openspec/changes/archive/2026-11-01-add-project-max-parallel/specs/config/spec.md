## ADDED Requirements

### Requirement: 並列数の設定
プロジェクトの設定は、整数の `<projects[].max_parallel>` を持てなければならない（MUST）。無いときは、そのプロジェクトに独自の上限は無い。
`<projects[].max_parallel>` が1より小さいとき、`switchboard` は設定の誤りとして拒まなければならない（MUST）。
`<projects[].max_parallel>` が `<agent.max_parallel>` より大きいとき、`switchboard` は設定の誤りにしてはならない（MUST NOT）。

#### Scenario: プロジェクトの上限が0
- **WHEN** `<projects[].max_parallel>` が0のプロジェクトを持つ設定を読む
- **THEN** 設定の誤りとして、そのプロジェクトの `max_parallel` が報告される

#### Scenario: プロジェクトの上限が全体より大きい
- **WHEN** `<agent.max_parallel>` が1で、`<projects[].max_parallel>` が3のプロジェクトを持つ設定を読む
- **THEN** 誤りにならず、そのプロジェクトで同時に動く実行は1つまでになる

## MODIFIED Requirements

### Requirement: 設定の読み直し
`switchboard agent run` と `switchboard inbox serve` は、動いている間、10秒ごとに設定ファイルの更新の時刻と中身を確かめなければならない（MUST）。
中身が変わり、検証を通ったとき、`switchboard` は `projects` を新しい設定に入れ替えなければならない（MUST）。再起動は要らない。
中身が変わり、検証を通ったとき、`switchboard agent run` は `<agent.max_parallel>` も新しい値に入れ替えなければならない（MUST）。

#### Scenario: 手でプロジェクトを足した
- **WHEN** `switchboard agent run` が動いている間に、`config.yaml` の `projects` に手で1項目を足して保存する
- **THEN** 10秒ほどのうちに、エージェントはそのプロジェクトを扱い始める

#### Scenario: 全体の上限を変えた
- **WHEN** `switchboard agent run` が動いている間に、`<agent.max_parallel>` を1から2に変えて保存する
- **THEN** 10秒ほどのうちに、エージェントは2つの実行を同時に動かせるようになり、再起動を求めるログは残らない

### Requirement: projects以外の変化
読み直した設定で、`projects` と `<agent.max_parallel>` 以外の項目が変わっているとき、`switchboard` はその項目を反映してはならない（MUST NOT）。再起動するまで反映しないことを、変わった節の名前とともにログに残す。

#### Scenario: 間隔を変えた
- **WHEN** `switchboard agent run` の動作中に、`<agent.poll_interval>` を変えて保存する
- **THEN** ポーリングの間隔は変わらない。`agent` の変化は再起動まで反映しないと、ログに残る
