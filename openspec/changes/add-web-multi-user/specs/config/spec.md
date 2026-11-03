# Spec Delta

## ADDED Requirements

### Requirement: 受信箱の認証の設定
`config.yaml` の `<inbox.auth>` に、ログインできるGitLabのグループと、ownerを書けなければならない（MUST）。
`<member_groups>` と `<admin_groups>` はGitLabのグループのフルパスの列、`<owner>` はGitLabのユーザー名1つとする。
`<inbox.auth>` を書いたとき、次のどちらかなら設定の誤りとしなければならない（MUST）。

- `forges.gitlab` が無い
- `<member_groups>`、`<admin_groups>`、`<owner>` がどれも空である

#### Scenario: GitLabの無い構成で認証を書く
- **WHEN** `forges.gitlab` の無い設定に `<inbox.auth>` を書く
- **THEN** コマンドは0以外の終了コードで終わり、認証にはGitLabの設定が要ると報告する

### Requirement: 認証を設定したときのエージェントの身元
`<inbox.auth>` を書いたとき、エージェントの身元は、同じフォージの `trusted_authors` にあってはならない（MUST NOT）。エージェントの身元は、フォージの `agent_identity` と、プロジェクトごとの `agent_identity` である。あれば、設定の誤りとしなければならない（MUST）。

#### Scenario: エージェントが人のアカウントで投稿する構成
- **WHEN** `<inbox.auth>` があり、`forges.gitlab.agent_identity` と `trusted_authors.gitlab` に同じ `alice` がある
- **THEN** コマンドは0以外の終了コードで終わり、認証を使うときはエージェントにbotの身元が要ると報告する

### Requirement: 認証の設定の検査
`switchboard doctor` は、`<inbox.auth>` を書いたとき、次の場合に警告を出さなければならない（MUST）。

- GitLabの共有の受信箱のトークンが、書き込みのスコープ（`api`）を持つ
- `<owner>` が `trusted_authors.gitlab` に無い
- GitHubのプロジェクトがあり、GitHubの受信箱のトークンの持ち主が `trusted_authors.github` に無い

#### Scenario: 共有のトークンがapiのスコープを持つ
- **WHEN** `<inbox.auth>` を書き、GitLabの `inbox_token_ref` のトークンが `api` のスコープを持つ
- **THEN** `switchboard doctor` は、共有のトークンを `read_api` にするよう警告する。終了コードは警告で変わらない
