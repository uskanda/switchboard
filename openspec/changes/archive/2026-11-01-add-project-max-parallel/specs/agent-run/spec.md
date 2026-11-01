## MODIFIED Requirements

### Requirement: 同時実行の制限
エージェントは、同じissueについて同時に2つ以上の実行をしてはならない（MUST NOT）。
エージェントは、端末で同時に動かす実行の数を `<agent.max_parallel>` 以下に保たなければならない（MUST）。
`<projects[].max_parallel>` のあるプロジェクトでは、エージェントは、同時に動かす実行の数をその値以下に保たなければならない（MUST）。

#### Scenario: 上限に達している
- **WHEN** `<agent.max_parallel>` が1の端末で1つの実行が動いている間に、別のissueに `agent:ready` が付く
- **THEN** 2つ目の実行は、1つ目が終わってから始まる

#### Scenario: プロジェクトの上限に達している
- **WHEN** 全体の上限が3、プロジェクトAの上限が1の端末で、Aの実行が1つ動いている。その間に、Aの別のissueに `agent:ready` が付く
- **THEN** 2つ目のAの実行は、1つ目が終わってから始まる

#### Scenario: プロジェクトの上限を書いていない
- **WHEN** 全体の上限が2の端末で、`<projects[].max_parallel>` の無いプロジェクトAの実行が1つ動いている。その間に、Aの別のissueに `agent:ready` が付く
- **THEN** 2つ目のAの実行はすぐに始まる

## ADDED Requirements

### Requirement: 順番待ちの取り出し
実行が終わったとき、エージェントは、上限に達していないプロジェクトの順番待ちのissueを実行しなければならない（MUST）。
上限に達したプロジェクトの順番待ちのissueは、ほかのプロジェクトの順番待ちのissueの実行を妨げてはならない（MUST NOT）。

#### Scenario: 前のissueのプロジェクトが上限に達している
- **WHEN** 全体の上限が2、プロジェクトAの上限が1の端末で、Aの実行が1つ動いている。順番待ちにはAのissueとプロジェクトBのissueがある。そこでBの別の実行が終わる
- **THEN** Bの順番待ちのissueの実行が始まり、Aの順番待ちのissueは順番待ちのまま残る

### Requirement: 上限の読み直し
設定の読み直しで上限が変わったとき、エージェントは、再起動せずに新しい上限で順番待ちのissueを実行しなければならない（MUST）。
上限とは、`<agent.max_parallel>` と `<projects[].max_parallel>` をいう。
上限が下がったとき、エージェントは、動いている実行を止めてはならない（MUST NOT）。

#### Scenario: 全体の上限を上げた
- **WHEN** `<agent.max_parallel>` が1の端末で1つの実行が動き、別のissueが順番待ちになっている。その間に `<agent.max_parallel>` を2に変えて保存する
- **THEN** エージェントは再起動せず、順番待ちのissueの実行を始める

#### Scenario: 全体の上限を下げた
- **WHEN** `<agent.max_parallel>` が2の端末で2つの実行が動いている間に、`<agent.max_parallel>` を1に変えて保存する
- **THEN** 2つの実行は最後まで続き、両方が終わるまで新しい実行は始まらない
