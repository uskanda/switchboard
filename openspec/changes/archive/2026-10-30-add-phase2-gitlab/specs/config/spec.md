## REMOVED Requirements

### Requirement: 対応するフォージ
**Reason**: Phase 2でGitLabに対応するため、`gl:` のプロジェクトを拒否する規則は要らなくなる。
**Migration**: 次の要件「フォージとプロジェクトの書き方」に置き換える。`gl:` のプロジェクトを書いた設定は、GitLabの設定があれば受け付けられる。

## ADDED Requirements

### Requirement: フォージとプロジェクトの書き方
`switchboard` は、`gh:` で始まるプロジェクトと、`gl:` で始まるプロジェクトを受け付けなければならない（MUST）。
`gh:` のプロジェクトは `gh:owner/repo` の形、`gl:` のプロジェクトは `gl:group/project` か、サブグループを含む `gl:group/sub/project` の形とする。
設定にあるプロジェクトのフォージについて、`forges` にそのフォージの設定が無いとき、`switchboard` は何も変更せずに終了し、足りない設定を報告しなければならない（MUST）。

#### Scenario: サブグループのプロジェクト
- **WHEN** `projects` に `ref: gl:nas/tools/switchboard-sandbox` があり、`forges` にGitLabの設定がある
- **THEN** 設定は受け付けられ、プロジェクトのパスは `nas/tools/switchboard-sandbox` として扱われる

#### Scenario: GitLabの設定が無い
- **WHEN** `projects` に `ref: gl:nas/switchboard-sandbox` があり、`forges` にGitLabの設定が無い
- **THEN** コマンドは0以外の終了コードで終わり、GitLabの設定が無いと報告する

### Requirement: GitLabのインスタンス
`switchboard` は、GitLabの設定の `base_url` が指すGitLabを使わなければならない（MUST）。
`base_url` が無いとき、その値は `https://gitlab.com` とする。
この版の設定に書けるGitLabは1つとする。

#### Scenario: セルフホストのGitLab
- **WHEN** GitLabの設定に `base_url: https://gl.uskan.dev` がある
- **THEN** GitLabのAPIは `https://gl.uskan.dev` に対して呼ばれる

#### Scenario: 既定のGitLab
- **WHEN** GitLabの設定に `base_url` が無い
- **THEN** GitLabのAPIは `https://gitlab.com` に対して呼ばれる

### Requirement: プロジェクトごとのエージェントの身元
プロジェクトの設定に `token_ref` があるとき、エージェントは、そのプロジェクトではプロジェクトの `token_ref` のトークンを使わなければならない（MUST）。フォージの `token_ref` は使わない。
プロジェクトの設定に `agent_identity` があるとき、`switchboard` は、そのプロジェクトではプロジェクトの `agent_identity` を使わなければならない（MUST）。
`switchboard inbox`、`switchboard show`、`switchboard reply` は、フォージの `inbox_token_ref` のトークンを使わなければならない（MUST）。プロジェクトの上書きは、受信箱には使わない。
この上書きは、GitHubとGitLabのどちらのプロジェクトにも使える。

#### Scenario: Project Access Tokenのbotで投稿する
- **WHEN** GitLabのプロジェクトに `token_ref: switchboard/gitlab-sandbox-bot` と、そのbotのユーザー名の `agent_identity` が設定されている
- **THEN** そのプロジェクトでは、エージェントのコメント、ラベルの付け替え、push、MRの作成に、そのbotのトークンが使われる

#### Scenario: 受信箱は上書きを使わない
- **WHEN** プロジェクトに `token_ref` の上書きがある設定で `switchboard reply 1 "..."` を実行する
- **THEN** 返答は、フォージの `inbox_token_ref` のトークンで投稿される
