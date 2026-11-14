## MODIFIED Requirements

### Requirement: 起動の環境
エージェントは、許可リストにある環境変数だけを `claude -p` に渡さなければならない（MUST）。
許可リストは `PATH`、`HOME`、`LANG`、`TMPDIR`、`SHELL`、`USER` と、プロキシとCA証明書の変数とする。
`auth: api_key` の設定のときに限り、エージェントは `ANTHROPIC_API_KEY` も渡さなければならない（MUST）。
エージェントは、フォージのトークン、`SSH_AUTH_SOCK`、`CLAUDE_CODE_SESSION_ID` を `claude -p` に渡してはならない（MUST NOT）。
エージェントは、`claude -p` の環境の `GOCACHE` を、実行のGoのビルドキャッシュの置き場所にしなければならない（MUST）。親の環境の `GOCACHE` は渡さない。

#### Scenario: 親の環境にトークンがある
- **WHEN** `GH_TOKEN` と `SSH_AUTH_SOCK` が設定されたシェルで `switchboard agent run` を起動する
- **THEN** `claude -p` の環境には、どちらの変数も無い

#### Scenario: 既定の認証方式
- **WHEN** `auth: subscription` の設定で、親の環境に `ANTHROPIC_API_KEY` がある
- **THEN** `claude -p` の環境に `ANTHROPIC_API_KEY` は無い

#### Scenario: 親の環境のGOCACHE
- **WHEN** 親の環境に `GOCACHE=/home/u/.cache/go-build` がある状態で、実装の実行を始める
- **THEN** `claude -p` の環境の `GOCACHE` は、実行のGoのビルドキャッシュの置き場所であり、`/home/u/.cache/go-build` ではない

## ADDED Requirements

### Requirement: 実行のGoのビルドキャッシュ
実行のGoのビルドキャッシュの置き場所は、利用者のキャッシュのディレクトリの下の `switchboard/go-build` でなければならない（MUST）。置き場所は全プロジェクトで1つとする。
エージェントは、`claude -p` を起動する前に、置き場所を作らなければならない（MUST）。
エージェントは、sandboxの中のコマンドに、置き場所への書き込みを許さなければならない（MUST）。worktreeと `$TMPDIR` のほかに書き込みを許すのは、この置き場所だけとする。
エージェントは、準備のコマンドと検証コマンドを、同じ `GOCACHE` で実行しなければならない（MUST）。
エージェントは、利用者自身のGoのビルドキャッシュへの書き込みを、sandboxの中のコマンドに許してはならない（MUST NOT）。

#### Scenario: 実行の中でテストを動かす
- **WHEN** Goのプロジェクトの実装の実行の中で、Claudeが `go test ./...` を実行する
- **THEN** コマンドは `read-only file system` で失敗せず、ビルドの結果は置き場所に残る

#### Scenario: 次の実行がキャッシュを使う
- **WHEN** 前の実行が `go test ./...` を実行したプロジェクトで、次の実行のClaudeが同じコマンドを実行する
- **THEN** 前の実行が置き場所に残したビルドの結果が使われる

#### Scenario: 計画の実行でビルドする
- **WHEN** Goのプロジェクトの計画の実行の中で、Claudeが前置きの変数を付けずに `go build ./...` を実行する
- **THEN** コマンドは `read-only file system` で失敗しない

#### Scenario: 利用者のキャッシュに書く
- **WHEN** 実装の実行の中で、Claudeが `touch ~/.cache/go-build/x` を実行する
- **THEN** コマンドは失敗し、ファイルは作られない

#### Scenario: 置き場所がまだ無い
- **WHEN** `~/.cache/switchboard/go-build` が無い端末で、実装の実行を始める
- **THEN** `claude -p` の起動の前に置き場所が作られ、実行の中の `go build ./...` は成功する

### Requirement: Goのビルドキャッシュの指示
エージェントは、sandboxの中での作業の指示に、`GOCACHE` が設定済みであることと、書き換えないことを書かなければならない（MUST）。

#### Scenario: 実装の実行の指示
- **WHEN** 実装の実行を始める
- **THEN** `--append-system-prompt-file` のファイルに、`GOCACHE` を書き換えないように書かれている
