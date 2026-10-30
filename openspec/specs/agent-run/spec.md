# agent-run Specification

## Purpose

端末で常駐するエージェントが、担当するissueの更新を検知して `claude -p` を起動する。プロンプトには信頼できる入力だけを使う。実行の停止と、端末の再起動後の回復も受け持つ。

## Requirements

### Requirement: ポーリングによる検知
`switchboard agent run` の間、エージェントは、`poll_interval` ごとに担当プロジェクトの更新されたissueを取得しなければならない（MUST）。
エージェントは、前回の取得以後に更新されたissueだけを評価しなければならない（MUST）。

#### Scenario: 更新が無い
- **WHEN** 前回の取得以後に、担当プロジェクトのどのissueも更新されていない
- **THEN** エージェントは実行を始めず、issueにも書き込まない

### Requirement: 着手
`agent:ready` が付いたopenのissueがあり、信頼判定を通ったとき、エージェントは、状態を `agent:running` に変えてから実行を始めなければならない（MUST）。

#### Scenario: 着手する
- **WHEN** マスターが作ったissueに、マスターが `agent:ready` を付ける
- **THEN** 次のポーリングでissueは `agent:running` になり、`claude -p` が起動する

### Requirement: 返答による再開
`agent:waiting` のissueに新しい信頼済みコメントがあるとき、エージェントは、状態を `agent:running` に変えてから実行を始めなければならない（MUST）。
新しいコメントとは、最新のマーカーの `ack` が指すコメントより後のコメントをいう。

#### Scenario: 返答で再開する
- **WHEN** 待ちのissueに、マスターが返答のコメントを書く
- **THEN** 次のポーリングでissueは `agent:running` になり、返答を含むプロンプトで `claude -p` が起動する

### Requirement: 完了後のコメント
`agent:done` のissueに信頼済みのコメントが付いたとき、エージェントは実行を始めてはならない（MUST NOT）。

#### Scenario: 完了したissueへのコメント
- **WHEN** `agent:done` のissueに、マスターがコメントする
- **THEN** 実行は始まらず、issueの状態は `agent:done` のままである

### Requirement: 同時実行の制限
エージェントは、同じissueについて同時に2つ以上の実行をしてはならない（MUST NOT）。
エージェントは、同時に動かす実行の数を `max_parallel` 以下に保たなければならない（MUST）。

#### Scenario: 上限に達している
- **WHEN** `max_parallel: 1` の端末で1つの実行が動いている間に、別のissueに `agent:ready` が付く
- **THEN** 2つ目の実行は、1つ目が終わってから始まる

### Requirement: 作業場所
エージェントは、issueごとに専用のgit worktreeを作り、`switchboard/issue-<n>` ブランチで `claude -p` を実行しなければならない（MUST）。
worktreeは、プロジェクトの `base_branch` から作る。

#### Scenario: 初めての実行
- **WHEN** issue 12に初めて着手する
- **THEN** `base_branch` から `switchboard/issue-12` ブランチのworktreeが作られ、そこが `claude -p` の作業ディレクトリになる

### Requirement: Claude Codeの設定の検査
実行を始める前に、エージェントは、worktreeの `.claude/`、`.mcp.json`、`CLAUDE.md`、`AGENTS.md` を `base_branch` と比べなければならない（MUST）。
1つでも異なるとき、エージェントは `claude -p` を起動せず、issueを `waiting(reason=verify)` にしなければならない（MUST）。

#### Scenario: 前回の実行がフックを書き足した
- **WHEN** 前回の実行で、worktreeの `.claude/settings.json` が書き換えられている
- **THEN** `claude -p` は起動せず、issueは待ちになり、設定の不一致を説明するコメントが付く

### Requirement: 起動の環境
エージェントは、許可リストにある環境変数だけを `claude -p` に渡さなければならない（MUST）。
許可リストは `PATH`、`HOME`、`LANG`、`TMPDIR`、`SHELL`、`USER` と、プロキシとCA証明書の変数とする。
`auth: api_key` の設定のときに限り、エージェントは `ANTHROPIC_API_KEY` も渡さなければならない（MUST）。
エージェントは、フォージのトークン、`SSH_AUTH_SOCK`、`CLAUDE_CODE_SESSION_ID` を `claude -p` に渡してはならない（MUST NOT）。

#### Scenario: 親の環境にトークンがある
- **WHEN** `GH_TOKEN` と `SSH_AUTH_SOCK` が設定されたシェルで `switchboard agent run` を起動する
- **THEN** `claude -p` の環境には、どちらの変数も無い

#### Scenario: 既定の認証方式
- **WHEN** `auth: subscription` の設定で、親の環境に `ANTHROPIC_API_KEY` がある
- **THEN** `claude -p` の環境に `ANTHROPIC_API_KEY` は無い

### Requirement: 毎回新しいセッション
エージェントは、実行ごとに新しいセッションIDを決め、`--session-id` で指定して `claude -p` を起動しなければならない（MUST）。
この版のエージェントは、`--resume` で以前のセッションを再開してはならない（MUST NOT）。

#### Scenario: 返答による2回目の実行
- **WHEN** 待ちのissueへの返答で2回目の実行を始める
- **THEN** 1回目とは別のセッションIDで起動し、issueの本文と信頼済みのコメントの全体をプロンプトに含める

### Requirement: プロンプトの内容
エージェントは、issueのタイトルと本文、信頼済みのコメント、エージェント自身の過去の投稿から、プロンプトを作らなければならない（MUST）。
プロンプトのうち、エージェントが追記する指示（終了の宣言の方法、質問の書き方、禁止する操作）は、`--append-system-prompt-file` で渡さなければならない（MUST）。

#### Scenario: プロンプトの構成
- **WHEN** 本文とコメント3件（信頼済み2件、非信頼1件）を持つissueで実行を始める
- **THEN** プロンプトには本文と信頼済みのコメント2件だけが入る

### Requirement: タイムアウト
実行が `timeout` を超えたとき、エージェントは、`claude -p` のプロセスグループにSIGINTを送り、猶予の後にSIGTERM、SIGKILLの順で止めなければならない（MUST）。
止めた実行は、`waiting(reason=error)` として扱わなければならない（MUST）。

#### Scenario: 終わらない実行
- **WHEN** `timeout: 45m` の設定で、実行が45分を超えて続く
- **THEN** プロセスが止まり、issueは待ちになり、タイムアウトを説明するコメントが付く

### Requirement: 実行中の取り消し
実行中に、issueがcloseされたとき、または `agent:running` が外されたとき、エージェントは実行を止めなければならない（MUST）。
取り消しで止めた実行について、エージェントはissueのラベルを変えてはならない（MUST NOT）。

#### Scenario: マスターがラベルを外した
- **WHEN** 実行中のissueから、マスターが `agent:running` を外す
- **THEN** 次のポーリングで実行が止まり、issueのラベルはマスターが変えたままである

### Requirement: 実行中に届いた返答
実行中に信頼済みのコメントが届き、その実行が待ちで終わったとき、エージェントは、待たずに次の実行を始めなければならない（MUST）。

#### Scenario: 行き違った返答
- **WHEN** 実行中にマスターが返答を書き、その実行が質問を残して終わる
- **THEN** issueは待ちを経て、直ちに `agent:running` に戻り、返答を含むプロンプトで次の実行が始まる

### Requirement: 再起動後の回復
`switchboard agent run` の起動時に、実行中のプロセスが無い `agent:running` のissueがあるとき、エージェントはそのissueを待ちにしなければならない（MUST）。
待ちの理由は `error` とする。

#### Scenario: 端末の再起動
- **WHEN** 実行中に端末が再起動し、その後 `switchboard agent run` を起動する
- **THEN** 中断されたissueは待ちになり、中断を説明するコメントが付く

### Requirement: 状態の表示
`switchboard agent status` は、この端末の担当プロジェクトと、実行中のissue、最後の実行の結果を表示しなければならない（MUST）。

#### Scenario: 実行中の表示
- **WHEN** issue 12の実行中に `switchboard agent status` を実行する
- **THEN** issue 12が実行中として、開始時刻とともに表示される
