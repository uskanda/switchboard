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
`switchboard` は、`token_ref` に対応するsystemdのクレデンシャルがあれば、その中身をトークンとして使わなければならない（MUST）。クレデンシャルは、環境変数 `CREDENTIALS_DIRECTORY` が指すディレクトリから探す。
クレデンシャルの名前は、`token_ref` の英数字以外の文字を `_` に置き換えた文字列とする。
クレデンシャルが無いとき、`switchboard` は、`token_ref` が指す項目をSecret Serviceから取り出してトークンとして使わなければならない（MUST）。
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

#### Scenario: systemdのクレデンシャル
- **WHEN** systemdのサービスとして動き、`$CREDENTIALS_DIRECTORY/switchboard_github` がある
- **THEN** その中身がトークンとして使われ、Secret Serviceには問い合わせない

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

### Requirement: 中継の設定
設定の `<relay>` 節は、中継のwebhookの受け口とSSEの配信口のアドレス、webhookの一覧、端末のトークンの一覧を持つ。
webhookの一覧の各項目は、プロジェクトの `ref`、`hook-key`、シークレットの `secret_ref` を持つ。GitLabのプロジェクトでは、`<gitlab_token>` を持ってよい（MAY）。
端末のトークンの一覧の各項目は、端末名とトークンのハッシュを持つ。
設定の検証は、`<relay>` 節にシークレットやトークンの値と見える文字列があれば、設定の誤りとして拒否しなければならない（MUST）。ハッシュは、決まった接頭辞で値と見分ける。

#### Scenario: hook-keyの重複
- **WHEN** `<relay>` 節の2つのwebhookが同じ `hook-key` を持つ
- **THEN** 設定の検証は誤りを報告し、`relay serve` は起動しない

### Requirement: エージェントの中継モードの設定
設定の `agent` は、`<relay>`（中継のSSEのURLと、bearerトークンの `token_ref`）と `<reconcile_interval>` を持ってよい（MAY）。
`<relay>` が無いとき、エージェントは今までどおり `poll_interval` ごとのポーリングだけで動かなければならない（MUST）。
`<reconcile_interval>` は `poll_interval` 以上でなければならない（MUST）。

#### Scenario: 中継の設定が無い
- **WHEN** `agent` に `<relay>` が無い設定で、`switchboard agent run` を実行する
- **THEN** エージェントは中継につながず、`poll_interval` ごとにポーリングする

### Requirement: 通知の設定
設定は、`<notify.telegram>` に、`chat_id` と `token_ref` を持てなければならない（MUST）。片方だけのときは、設定の誤りとして拒む。
`<notify.web_url>` は、WUIのURLで、`http` か `https` の絶対URLでなければならない（MUST）。無くてもよい。
`<notify>` の節が無い設定は、今までどおり読めなければならない（MUST）。

#### Scenario: chat_idだけがある
- **WHEN** `<notify.telegram>` に `chat_id` だけがあり、`token_ref` が無い設定を読む
- **THEN** 設定の誤りとして報告される

#### Scenario: 通知の節が無い
- **WHEN** `<notify>` の無い設定を読む
- **THEN** 誤りにならず、通知は無効になる

### Requirement: 計画のモードの設定
プロジェクトの設定は、真偽値の `<projects[].plan_first>` を持てなければならない（MUST）。無いときは偽とする。
設定は、`<plan.approve_word>` に承認の語を持てなければならない（MUST）。無いときは `/ok` とする。空の文字列や、空白を含む文字列は、設定の誤りとして拒む。
設定の `labels` は、`<labels.plan>` と `<labels.go>` を持てなければならない（MUST）。既定の名前は、それぞれ `agent:plan`、`agent:go` である。
これらの項目が無い設定は、今までどおり読めなければならない（MUST）。

#### Scenario: 承認の語に空白がある
- **WHEN** `<plan.approve_word>` が `ok please` の設定を読む
- **THEN** 設定の誤りとして、`<plan.approve_word>` が報告される

#### Scenario: 項目が無い設定
- **WHEN** `<projects[].plan_first>`、`<plan.approve_word>`、`<labels.plan>`、`<labels.go>` のどれも無い設定を読む
- **THEN** 誤りにならず、どのプロジェクトでも `agent:ready` ですぐに実装に入る

### Requirement: 受信箱の更新の間隔の設定
`<inbox.refresh_interval>` は、受信箱のサービスが、画面を開いている間にフォージから一覧を取り直す間隔でなければならない（MUST）。
`<inbox.refresh_interval>` が無いとき、間隔は30秒としなければならない（MUST）。
`<inbox.refresh_interval>` が10秒より短いとき、設定の検証は、設定を拒まなければならない（MUST）。

#### Scenario: 書かない
- **WHEN** `inbox` の節が無い設定で `switchboard inbox serve` を起動する
- **THEN** 一覧は30秒ごとに取り直される

#### Scenario: 短すぎる
- **WHEN** `<inbox.refresh_interval>` に `5s` を書いて設定を検証する
- **THEN** 設定は拒まれ、誤りには下限の10秒が示される

### Requirement: プロジェクトの追加の親ディレクトリ
設定は、`<inbox.project_root>` に、追加したプロジェクトのcloneを置く親ディレクトリを持てなければならない（MUST）。先頭の `~` は、利用者のホームディレクトリとして読む。
`~` を読んだあとの値が絶対パスでないとき、`switchboard` は設定の誤りとして拒まなければならない（MUST）。
`<inbox.project_root>` が無い設定は、今までどおり読めなければならない（MUST）。そのとき、プロジェクトの追加は無効になる。

#### Scenario: 相対パス
- **WHEN** `<inbox.project_root>` が `repos` の設定を読む
- **THEN** 設定の誤りとして、`<inbox.project_root>` が報告される

#### Scenario: キーが無い
- **WHEN** `<inbox.project_root>` の無い設定を読む
- **THEN** 誤りにならず、プロジェクトの追加は無効になる

### Requirement: プロジェクトの追加の書き込み
プロジェクトを追加するとき、`switchboard` は、設定ファイルの `projects` の末尾に1項目を足さなければならない（MUST）。ほかの項目、コメント、キーの並びは変えない。
設定ファイルがシンボリックリンクのとき、`switchboard` はリンク先のファイルを書き換えなければならない（MUST）。リンクは残す。

#### Scenario: コメントのある設定に足す
- **WHEN** コメントを含む設定ファイルに、`gh:uskanda/notes` を追加する
- **THEN** `projects` の末尾に `gh:uskanda/notes` の項目が足され、元のコメントと項目はそのまま残る

#### Scenario: シンボリックリンクの設定
- **WHEN** `config.yaml` が別のファイルへのシンボリックリンクで、プロジェクトを追加する
- **THEN** リンク先のファイルに項目が足され、`config.yaml` はシンボリックリンクのまま残る

### Requirement: 書き込みの前の検証
プロジェクトを足したあとの設定が、設定の検証を通らないとき、`switchboard` は設定ファイルを書き換えてはならない（MUST NOT）。

#### Scenario: 足すと誤りになる
- **WHEN** 手で書いた設定に検証を通らない項目があり、その状態でプロジェクトを追加する
- **THEN** 設定ファイルは変わらず、誤りのあるキーが報告される

### Requirement: 手の編集との衝突
追加のために設定ファイルを読んでから書くまでに、その中身が変わったとき、`switchboard` は設定ファイルを書き換えてはならない（MUST NOT）。誤りとして、設定ファイルが別に変更されたことを返す。

#### Scenario: 追加の途中で手で編集した
- **WHEN** 追加の処理がフォージに実在を問い合わせている間に、利用者が設定ファイルを手で保存する
- **THEN** 項目は足されず、設定ファイルの別の変更を示す誤りが報告される。手で保存した内容は残る

### Requirement: 元の設定の退避
設定ファイルを書き換える前に、`switchboard` は、元の中身を、設定ファイルと同じディレクトリの `<config.yaml.bak>` に保存しなければならない（MUST）。前の退避があれば、上書きする。
書き換えは、一時ファイルに書いてから置き換える形で行わなければならない（MUST）。書き換えの途中で止まっても、設定ファイルは元の中身か新しい中身のどちらかである。

#### Scenario: 追加のあとの退避
- **WHEN** プロジェクトを追加する
- **THEN** `<config.yaml.bak>` に追加の前の中身が残る

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

### Requirement: 読み直しの誤り
読み直した設定が検証を通らないとき、`switchboard` は前の設定のまま動き続けなければならない（MUST）。誤りのあるキーをログに残す。

#### Scenario: 書きかけの設定
- **WHEN** `switchboard agent run` が動いている間に、`projects` の項目から `path` を消して保存する
- **THEN** エージェントは前の設定のまま動き続け、ログには `path` の欠落を残す

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

### Requirement: 自動で直す回数の設定
設定の `agent` は、整数の `<agent.max_fix_attempts>` を持てなければならない（MUST）。無いときは2とする。
`<agent.max_fix_attempts>` が0のとき、エージェントは、CIの失敗とnon-fast-forwardを自動では直さない。
`<agent.max_fix_attempts>` が負のとき、`switchboard` は設定の誤りとして拒まなければならない（MUST）。

#### Scenario: 項目が無い設定
- **WHEN** `<agent.max_fix_attempts>` の無い設定を読む
- **THEN** 誤りにならず、自動で直す回数の上限は2になる

#### Scenario: 負の値
- **WHEN** `<agent.max_fix_attempts>` が-1の設定を読む
- **THEN** 設定の誤りとして、`<agent.max_fix_attempts>` が報告される
