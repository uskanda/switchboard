# Spec Delta

## ADDED Requirements

### Requirement: 準備のコマンドの実行
プロジェクトに `<projects[].setup_command>` があるとき、エージェントは、`claude -p` の起動の前に準備のコマンドをworktreeで実行しなければならない（MUST）。
エージェントは、準備のコマンドを、実行のたびに、worktreeの準備のあとで実行する。
対象は、計画の実行、承認のあとの実行、直す実行を含むすべての実行とする。
`<projects[].setup_command>` が無いプロジェクトでは、エージェントは、今までどおり準備のコマンドなしで `claude -p` を起動する。

#### Scenario: 初めての実行
- **WHEN** `setup_command: pnpm install --frozen-lockfile` のプロジェクトで、issue 12に初めて着手する
- **THEN** issue 12のworktreeで `pnpm install --frozen-lockfile` が実行され、それが終わってから `claude -p` が起動する

#### Scenario: 同じissueの2回目の実行
- **WHEN** 同じプロジェクトで、issue 12の待ちに返答が付き、2回目の実行が始まる
- **THEN** 準備のコマンドがもう一度実行されてから `claude -p` が起動する

#### Scenario: 計画の実行
- **WHEN** 同じプロジェクトで、計画の実行が始まる
- **THEN** 準備のコマンドが実行されてから `claude -p` が起動する

#### Scenario: 準備のコマンドが無いプロジェクト
- **WHEN** `<projects[].setup_command>` が無いプロジェクトで実行が始まる
- **THEN** 準備のコマンドは実行されず、`claude -p` が起動する

### Requirement: 準備のコマンドの環境
エージェントは、準備のコマンドを、実行のsandboxの外で、検証コマンドと同じ環境変数で実行しなければならない（MUST）。
エージェントは、準備のコマンドにフォージのトークンを渡してはならない（MUST NOT）。

#### Scenario: ネットワークに接続する
- **WHEN** 準備のコマンドがパッケージのレジストリから依存を取得する
- **THEN** 接続は実行のsandboxに止められず、依存がworktreeに置かれる

#### Scenario: フォージのトークン
- **WHEN** エージェントの環境にフォージのトークンがあり、準備のコマンドが環境変数の一覧を出力する
- **THEN** 出力にフォージのトークンは含まれない

### Requirement: 準備のコマンドの失敗
準備のコマンドが0以外の終了コードで終わったとき、エージェントは、`claude -p` を起動せずに、実行を `waiting(reason=error)` として扱わなければならない（MUST）。
このとき、エージェントは、コメントに準備のコマンドと出力の末尾を書かなければならない（MUST）。

#### Scenario: レジストリに届かない
- **WHEN** 準備のコマンドが、レジストリへの接続に失敗して終了コード1で終わる
- **THEN** `claude -p` は起動せず、issueは待ちになり、コメントに準備のコマンドと出力の末尾が書かれる

### Requirement: 準備のコマンドが残した変更
準備のコマンドが終了コード0で終わったとき、エージェントは、worktreeのgitの状態が準備のコマンドの前と同じであることを確かめなければならない（MUST）。
gitの状態とは、追跡しているファイルの差分と、無視の対象でない追跡されていないファイルをいう。
gitの状態が変わっていたとき、エージェントは、準備のコマンドの失敗として扱い、コメントに変わったパスを書かなければならない（MUST）。
エージェントは、準備のコマンドが残した変更を捨てない。

#### Scenario: 無視の対象のディレクトリだけが増える
- **WHEN** `.gitignore` に `node_modules` のあるプロジェクトで、準備のコマンドが `node_modules` だけを書いて終了コード0で終わる
- **THEN** `claude -p` が起動する

#### Scenario: ロックファイルを書き換える
- **WHEN** 準備のコマンドが、追跡している `pnpm-lock.yaml` を書き換えて終了コード0で終わる
- **THEN** `claude -p` は起動せず、issueは待ちになり、コメントに `pnpm-lock.yaml` が書かれる

#### Scenario: 前の実行の差分が残っている
- **WHEN** 前の実行がcommitしていない差分をworktreeに残していて、準備のコマンドがその差分を変えないまま終了コード0で終わる
- **THEN** `claude -p` が起動する

### Requirement: 準備のコマンドの時間の上限
準備のコマンドが `timeout` を超えたとき、エージェントは、準備のコマンドのプロセスグループを止め、準備のコマンドの失敗として扱わなければならない（MUST）。

#### Scenario: 終わらない準備のコマンド
- **WHEN** `timeout: 45m` の設定で、準備のコマンドが45分を超えて続く
- **THEN** 準備のコマンドとその子プロセスが止まり、`claude -p` は起動せず、issueは待ちになる

### Requirement: 準備のコマンドの取り消し
準備のコマンドの実行中に実行が取り消されたとき、エージェントは、準備のコマンドのプロセスグループを止め、`claude -p` を起動してはならない（MUST NOT）。

#### Scenario: 準備の途中でissueが閉じられる
- **WHEN** 準備のコマンドの実行中に、issueが閉じられて実行が取り消される
- **THEN** 準備のコマンドが止まり、`claude -p` は起動せず、失敗のコメントは付かない

### Requirement: 準備のコマンドの記録
準備のコマンドが終わったとき、エージェントは、監査ログに `<setup.ran>` を残さなければならない（MUST）。
`<setup.ran>` には、issue、実行のID、準備のコマンド、かかった時間、成否を入れ、出力は入れない。
エージェントは、準備のコマンドの出力の全体を、実行のディレクトリの `<setup.log>` に残さなければならない（MUST）。

#### Scenario: 成功した準備
- **WHEN** 準備のコマンドが終了コード0で終わる
- **THEN** 監査ログに、成功を示す `<setup.ran>` が1行残り、実行のディレクトリの `<setup.log>` に出力がある

#### Scenario: 失敗した準備
- **WHEN** 準備のコマンドが終了コード1で終わる
- **THEN** 監査ログに、失敗を示す `<setup.ran>` が1行残り、その行に出力は含まれない

### Requirement: 準備のコマンドの知らせ
プロジェクトに `<projects[].setup_command>` があるとき、エージェントは、追記する指示に、依存が準備のコマンドで用意済みであることを書かなければならない（MUST）。
指示には、準備のコマンドの文字列と、sandboxの中から依存を取得し直さないことを含める。

#### Scenario: 準備のコマンドのあるプロジェクトの指示
- **WHEN** `setup_command: pnpm install --frozen-lockfile` のプロジェクトで実行を始める
- **THEN** `--append-system-prompt-file` のファイルに、`pnpm install --frozen-lockfile` で依存を用意済みであることが書かれている

#### Scenario: 準備のコマンドが無いプロジェクトの指示
- **WHEN** `<projects[].setup_command>` が無いプロジェクトで実行を始める
- **THEN** `--append-system-prompt-file` のファイルに、準備のコマンドの項は無い

### Requirement: 準備のコマンドをClaudeに許さない
エージェントは、準備のコマンドを、`claude -p` がsandboxの外で実行できるコマンドに加えてはならない（MUST NOT）。

#### Scenario: Claudeが準備のコマンドを実行する
- **WHEN** `setup_command: pnpm install --frozen-lockfile` のプロジェクトの実行の中で、Claudeが同じコマンドを実行する
- **THEN** コマンドはsandboxの中で実行され、ネットワークに接続できない
