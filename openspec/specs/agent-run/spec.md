# agent-run Specification

## Purpose

端末で常駐するエージェントが、担当するissueの更新を検知して `claude -p` を起動する。プロンプトには信頼できる入力だけを使う。実行の停止と、端末の再起動後の回復も受け持つ。

## Requirements

### Requirement: ポーリングによる検知
`switchboard agent run` の間、エージェントは、`poll_interval` ごとに担当プロジェクトの更新されたissueを取得しなければならない（MUST）。
エージェントは、前回の取得以後に更新されたissueだけを評価しなければならない（MUST）。
GitLabのプロジェクトでは、エージェントは、ポーリングのたびに待ちのissueも取得しなければならない（MUST）。
人が書いたノートの数が前回の取得から変わった待ちのissueは、更新されたissueとして評価する。

#### Scenario: 更新が無い
- **WHEN** 前回の取得以後に、担当プロジェクトのどのissueも更新されていない
- **THEN** エージェントは実行を始めず、issueにも書き込まない

#### Scenario: GitLabの待ちへの返答
- **WHEN** GitLabの待ちのissueに、マスターが返答のノートを書く。issueの更新時刻は変わらない
- **THEN** 次のポーリングでノートの数の変化が検知され、issueは `agent:running` になる

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

### Requirement: 端末をまたいだ重複の防止
エージェントは、状態を `agent:running` へ変える前に、マーカー付きの着手のコメントを投稿しなければならない（MUST）。
着手のコメントのマーカーは、`state=running` と、端末の名前の `host`、実行の `run`、その実行が取り込む最後のコメントの `ack` を持つ。
投稿したあと、エージェントはissueのコメントを取り直し、自分と同じ `ack` を持つ着手のコメントのうち、コメントIDが最も小さいものを探す。
それが他の端末の着手のコメントのとき、エージェントは `claude -p` を起動してはならない（MUST NOT）。
退いたエージェントは、自分の着手のコメントを消し、ラベルを変えずに、退いたことをログと監査ログに記録しなければならない（MUST）。

#### Scenario: 2台が同時に着手する
- **WHEN** 端末 `ubuntu-a` と `ubuntu-b` が同じプロジェクトを担当し、同じissueの `agent:ready` を同じポーリングで見つける
- **THEN** 先に着手のコメントを投稿した端末だけが `claude -p` を起動する。もう一方は自分の着手のコメントを消し、何も実行しない

#### Scenario: 1台だけで着手する
- **WHEN** 1台の端末だけがissueに着手する
- **THEN** 着手のコメントが1件残り、そのまま `claude -p` が起動する

### Requirement: 着手のコメントと結果のコメント
エージェントは、同じ `run` と同じ `state` のマーカーのコメントがあるときだけ、結果を投稿済みとみなさなければならない（MUST）。
着手のコメント（`state=running`）があっても、エージェントは、その実行の結果のコメントを投稿しなければならない（MUST）。

#### Scenario: 着手のコメントのあとで待ちになる
- **WHEN** 着手のコメントを投稿した実行が、質問を残して終わる
- **THEN** 同じ `run` の質問のコメントが投稿され、issueは `agent:waiting` になる

### Requirement: 中継による検知
設定の `agent` に `<relay>` があるとき、`switchboard agent run` は、中継のSSEを購読しなければならない（MUST）。購読には、`<relay>` の `token_ref` が指すbearerトークンを使う。
エージェントは、届いたイベントのうち、この端末の担当プロジェクトのものだけを扱わなければならない（MUST）。同じプロジェクトのイベントは3秒まとめてから、そのプロジェクトの更新を取り直して評価する。取り直しには、ポーリングと同じ処理を使う。
エージェントは、イベントを取り直しの合図にだけ使い、イベントの中身でラベルやコメントを判断してはならない（MUST NOT）。同じイベントが2回届いても、結果は変わらない。
中継につながっている間、エージェントは、`<reconcile_interval>`（既定10分）ごとに担当の全プロジェクトをポーリングして照合しなければならない（MUST）。

#### Scenario: 返答がすぐに届く
- **WHEN** 中継につながったエージェントの担当する待ちのissueに、マスターが返答し、中継がそのイベントを配る
- **THEN** エージェントは、ポーリングを待たずに数秒でプロジェクトを取り直し、issueを `agent:running` にする

#### Scenario: 担当でないプロジェクト
- **WHEN** 中継が、この端末の担当でないプロジェクトのイベントを配る
- **THEN** エージェントはフォージに問い合わせず、何もしない

#### Scenario: 続けて届くイベント
- **WHEN** 同じプロジェクトのラベルの変更とコメントのイベントが、1秒の間に続けて届く
- **THEN** エージェントは、そのプロジェクトを1回だけ取り直す

### Requirement: 中継が使えないとき
中継につながらないか接続が切れたとき、エージェントは、すぐに `poll_interval` ごとのポーリングに戻らなければならない（MUST）。戻ったことはログに残す。
エージェントは、ポーリングを続けながら、間隔を空けて中継への再接続を試みなければならない（MUST）。再接続のときは、最後に受け取ったイベントのIDを `Last-Event-ID` に付ける。つながったら、担当の全プロジェクトを1回照合してから、照合の間隔に戻る。

#### Scenario: 中継が止まる
- **WHEN** 中継のサービスが止まる
- **THEN** エージェントは `poll_interval` ごとのポーリングで処理を続ける

#### Scenario: 中継が戻る
- **WHEN** 止まっていた中継が起動し直す
- **THEN** エージェントは再接続し、照合の間隔に戻る
