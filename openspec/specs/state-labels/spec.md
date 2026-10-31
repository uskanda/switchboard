# state-labels Specification

## Purpose

issueの状態を4つのラベルで表し、どのissueにも状態ラベルが常に1つだけ付いている状態を保つ。

## Requirements

### Requirement: ラベルの作成
`switchboard labels sync` は、設定されたすべてのプロジェクトに、6つの状態ラベルを作らなければならない（MUST）。
ラベルの名前は、設定の `labels` で変えられる。既定は `agent:ready`、`agent:plan`、`agent:go`、`agent:running`、`agent:waiting`、`agent:done` とする。`agent:plan` の名前は `<labels.plan>` で、`agent:go` の名前は `<labels.go>` で変える。
`agent:ready`、`agent:plan`、`agent:go` の3つを、始めるラベルと呼ぶ。始めるラベルは、どれも `ready` の状態を表す。
同じ名前のラベルが既にあるとき、`switchboard labels sync` は、そのラベルを作り直してはならない（MUST NOT）。

#### Scenario: 初めて実行する
- **WHEN** 状態ラベルが1つも無いリポジトリで `switchboard labels sync` を実行する
- **THEN** 6つの状態ラベルが作られる

#### Scenario: 2回目の実行
- **WHEN** 6つの状態ラベルが既にあるリポジトリで `switchboard labels sync` を実行する
- **THEN** ラベルは作られず、既存のラベルが付いたissueはそのままである

#### Scenario: この機能の前に同期したリポジトリ
- **WHEN** 4つの状態ラベルだけがあるリポジトリで `switchboard labels sync` を実行する
- **THEN** `agent:plan` と `agent:go` だけが作られる

### Requirement: 状態の付け替え
エージェントがissueの状態を変えるとき、エージェントは、ほかの状態ラベルを外して新しい状態ラベルを1つだけ付けなければならない（MUST）。
エージェントは、状態ラベル以外のラベルを変えてはならない（MUST NOT）。
付け替えの後、エージェントはissueを取り直し、状態ラベルが新しい1つだけであることを確かめなければならない（MUST）。

#### Scenario: 実行中から待ちへ
- **WHEN** `agent:running` と `bug` が付いたissueを、エージェントが待ちに変える
- **THEN** issueのラベルは `agent:waiting` と `bug` になる

#### Scenario: 付け替えの直後に人間がラベルを変えた
- **WHEN** 付け替えの後で取り直したissueに、状態ラベルが2つ付いている
- **THEN** エージェントは次の要件の規則で1つに戻し、監査ログに記録する

### Requirement: 複数の状態ラベルの正規化
状態ラベルが2つ以上付いたissueを見つけたとき、エージェントは、優先順位の最も高い1つだけを残さなければならない（MUST）。
優先順位は、高い順に `running`、`waiting`、`ready`、`done` とする。始めるラベルは、どれも `ready` の順位とする。
`ready` の順位が最も高く、始めるラベルが2つ以上付いているとき、エージェントは、どれも残さずにissueを `waiting(reason=error)` にしなければならない（MUST）。
そのとき、エージェントは、始めるラベルを1つにするよう求めるコメントを書く。

#### Scenario: readyとwaitingが付いている
- **WHEN** エージェントが `agent:ready` と `agent:waiting` の両方が付いたissueを見つける
- **THEN** `agent:ready` が外され、`agent:waiting` が残る

#### Scenario: planとgoが付いている
- **WHEN** マスターが、issueに `agent:plan` と `agent:go` の両方を付ける
- **THEN** 実行は始まらず、issueは `agent:waiting` になり、始めるラベルを1つにするよう求めるコメントが付く

#### Scenario: runningとplanが付いている
- **WHEN** エージェントが `agent:running` と `agent:plan` の両方が付いたissueを見つける
- **THEN** `agent:plan` が外され、`agent:running` が残る
