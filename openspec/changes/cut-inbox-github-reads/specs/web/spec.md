# Spec Delta

## ADDED Requirements

### Requirement: API制限の帯
一覧の `errors` の要素が `retry_at` を持つとき、WUIは、そのフォージの帯に、API制限を受けていることと、`retry_at` の時刻を表示しなければならない（MUST）。時刻は、ブラウザの時間帯で、時と分を表示する。
`retry_at` を持たない要素の帯は、今までどおり、フォージの名前と誤りの文を表示しなければならない（MUST）。
帯の文言は、日本語と英語の両方を持たなければならない（MUST）。

#### Scenario: 制限中に開く
- **WHEN** 時間帯がAsia/Tokyoのブラウザで、GitHubの要素の `retry_at` が07:49 UTCの一覧を表示する
- **THEN** 帯に、GitHubがAPI制限中であることと、16:49に読み直すことが表示され、一覧には前回のGitHubの行が出る

#### Scenario: 制限でない誤り
- **WHEN** GitLabの要素に `retry_at` が無く、誤りの文が `connection refused` の一覧を表示する
- **THEN** 帯に、GitLabを読めなかったことと `connection refused` が表示される
