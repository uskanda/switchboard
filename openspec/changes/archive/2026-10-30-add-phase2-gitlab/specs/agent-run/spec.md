## MODIFIED Requirements

### Requirement: ポーリングによる検知
`switchboard agent run` の間、エージェントは、`poll_interval` ごとに担当プロジェクトの更新されたissueを取得しなければならない（MUST）。
エージェントは、前回の取得以後に更新されたissueだけを評価しなければならない（MUST）。
GitLabのプロジェクトでは、エージェントは、ポーリングのたびに待ちのissueも取得しなければならない（MUST）。
人が書いたノートの数が前回の取得から変わった待ちのissueは、更新されたissueとして評価する。

#### Scenario: 更新が無い
- **WHEN** 前回の取得以後に、担当プロジェクトのどのissueも更新されていない
- **THEN** エージェントは実行を始めず、issueにも書き込まない

#### Scenario: GitLabの待ちへの返答
- **WHEN** GitLabの待ちのissueに、マスターが返答のノートを書く。issueの更新時刻は変わらない
- **THEN** 次のポーリングでノートの数の変化が検知され、issueは `agent:running` になる
