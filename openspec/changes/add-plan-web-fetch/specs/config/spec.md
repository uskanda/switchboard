## ADDED Requirements

### Requirement: 計画の実行のWebFetchの設定
`switchboard` は、真偽値の <agent.plan_web_fetch> と <projects[].plan_web_fetch> を読まなければならない（MUST）。
プロジェクトのWebFetchの設定は、<projects[].plan_web_fetch> があればその値とする。無ければ <agent.plan_web_fetch> の値とし、どちらも無ければオンとする。
<projects[].plan_web_fetch> の変化は、`projects` の読み直しとともに反映しなければならない（MUST）。<agent.plan_web_fetch> の変化は、再起動で反映する。
受信箱のAPIとWeb表示は、どちらの項目も変えられてはならない（MUST NOT）。

#### Scenario: どちらも書いていない
- **WHEN** <agent.plan_web_fetch> と <projects[].plan_web_fetch> のどちらも無い設定を読む
- **THEN** すべてのプロジェクトのWebFetchの設定はオンである

#### Scenario: 全体をオフにして1つだけオンにする
- **WHEN** <agent.plan_web_fetch> が `false` で、プロジェクトAだけ <projects[].plan_web_fetch> が `true` の設定を読む
- **THEN** プロジェクトAのWebFetchの設定はオンで、ほかのプロジェクトはオフである

#### Scenario: プロジェクトの値の読み直し
- **WHEN** エージェントが動いている間に、プロジェクトの <projects[].plan_web_fetch> を `false` に書き換える
- **THEN** 再起動しなくても、そのプロジェクトの次の計画の実行では `WebFetch` が拒否される
