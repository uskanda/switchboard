## MODIFIED Requirements

### Requirement: 一覧
APIは、`switchboard inbox --json` と同じ項目の待ちの一覧を返さなければならない（MUST）。並びと、設定に無いプロジェクトを出さないことも、`switchboard inbox` と同じにする。
一部のフォージに届かなかったときは、取得できた一覧と、届かなかったフォージとその理由を、ともに返す。
APIは、一覧を30秒キャッシュしてよい（MAY）。返答を投稿したら、キャッシュを捨てなければならない（MUST）。

#### Scenario: GitLabに届かない
- **WHEN** GitHubに待ちが1件あり、GitLabのAPIが失敗する状態で、一覧を求める
- **THEN** GitHubの1件と、GitLabに届かなかったことと理由が返る

#### Scenario: 続けて求める
- **WHEN** 一覧を求めてから30秒以内に、もう一度求める
- **THEN** フォージには問い合わせず、前の一覧が返る
