# config Specification

## Purpose

switchboardの各サブコマンドが共通に使う設定ファイルを読み込み、検証し、フォージのトークンを安全に取り出す。

## Requirements

### Requirement: 設定ファイルの場所
`switchboard` は、`--config` で指定されたファイルを設定として読み込まなければならない（MUST）。
`--config` が無いとき、`switchboard` は `$XDG_CONFIG_HOME/switchboard/config.yaml` を読み込まなければならない（MUST）。
`XDG_CONFIG_HOME` が無いとき、その場所は `~/.config` とする。

#### Scenario: 指定が無い
- **WHEN** `XDG_CONFIG_HOME` が未設定の環境で、`--config` を付けずに `switchboard agent status` を実行する
- **THEN** `~/.config/switchboard/config.yaml` が読み込まれる

#### Scenario: 指定がある
- **WHEN** `switchboard inbox --config ./test.yaml` を実行する
- **THEN** `./test.yaml` が読み込まれる

### Requirement: 設定の検証
設定に誤りがあるとき、`switchboard` は何も変更せずに終了し、誤りのあるキーを報告しなければならない（MUST）。
誤りには、必須のキーの欠落、型の不一致、未知のキーを含む。

#### Scenario: 必須のキーが無い
- **WHEN** `projects` の1つ目の要素に `path` が無い設定で `switchboard agent run` を実行する
- **THEN** コマンドは0以外の終了コードで終わり、1つ目のプロジェクトに `path` が無いと報告する

#### Scenario: 未知のキー
- **WHEN** 設定に `agnet:` という綴りの誤ったキーがある
- **THEN** コマンドは0以外の終了コードで終わり、`agnet` を未知のキーとして報告する

### Requirement: トークンの取り出し
`switchboard` は、`token_ref` が指す項目をSecret Serviceから取り出してトークンとして使わなければならない（MUST）。
設定の `secrets` の `allow_env` が `true` の間、Secret Serviceに項目が無いとき、`switchboard` は環境変数からトークンを読まなければならない（MUST）。
変数の名前は `SWITCHBOARD_TOKEN_` に、`token_ref` を英大文字にして英数字以外を `_` に置き換えた文字列を続けたものとする。

#### Scenario: Secret Serviceから取り出す
- **WHEN** Secret Serviceに `switchboard/github` の項目があり、設定の `token_ref` が `switchboard/github` である
- **THEN** その項目の値がトークンとして使われる

#### Scenario: 開発時の環境変数
- **WHEN** `allow_env: true` の設定で、Secret Serviceに項目が無く、`SWITCHBOARD_TOKEN_SWITCHBOARD_GITHUB` が設定されている
- **THEN** その環境変数の値がトークンとして使われる

#### Scenario: 環境変数が許されていない
- **WHEN** `allow_env` が無い設定で、Secret Serviceに項目が無い
- **THEN** コマンドは0以外の終了コードで終わり、取り出せない `token_ref` を報告する。環境変数は読まれない

### Requirement: 設定ファイルにトークンを書かせない
設定ファイルの値がフォージのトークンの形式に一致するとき、`switchboard` は、その値を使わずに終了しなければならない（MUST）。
形式は `ghp_`、`github_pat_`、`gho_`、`ghs_`、`glpat-` で始まる文字列とする。

#### Scenario: 平文のトークン
- **WHEN** 設定の `token_ref` に `ghp_` で始まる文字列が書かれている
- **THEN** コマンドは0以外の終了コードで終わり、設定にトークンを書かないよう報告する。報告にトークンの値は含まれない

### Requirement: 担当する端末
`switchboard agent run` は、`host` が端末の名前（設定の `host` の `name`）と一致するプロジェクトだけを扱わなければならない（MUST）。

#### Scenario: 他の端末の担当
- **WHEN** 端末の名前が `ubuntu-a` の端末で、`host: ubuntu-b` のプロジェクトにissueの更新がある
- **THEN** その端末のエージェントは、そのissueを取得せず、実行もしない

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
