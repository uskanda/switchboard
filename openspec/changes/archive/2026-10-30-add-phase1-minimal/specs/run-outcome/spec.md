## Purpose

`claude -p` の実行が終わったときに結果を判定し、待ちか完了かをフォージに書き戻す。フォージへの書き込みは、すべてエージェントが行う。

## ADDED Requirements

### Requirement: 終了の宣言
エージェントは、`--json-schema` で終了の宣言のスキーマを渡して `claude -p` を起動しなければならない（MUST）。
宣言は、`status`（`waiting` か `done`）と `summary` を必ず持ち、`waiting` のときは `question` も持つ。

#### Scenario: 質問を残して終わる
- **WHEN** Claudeが判断に迷い、`status: waiting` と質問を宣言して終わる
- **THEN** resultの `structured_output` から宣言が読み取られる

### Requirement: 異常な終了
次のどれかのとき、エージェントは実行を `waiting(reason=error)` として扱わなければならない（MUST）。
終了コードが0以外のとき、resultの `is_error` が真のとき、`structured_output` が無いかスキーマに合わないときが該当する。

#### Scenario: 最大ターン数に達した
- **WHEN** 実行が `max_turns` に達して終わる
- **THEN** issueは待ちになり、コメントに理由が書かれる

### Requirement: 待ちの書き戻し
実行が `waiting` を宣言したとき、エージェントは、質問をissueにコメントし、状態を `agent:waiting` に変えなければならない（MUST）。
resultの `permission_denials` が空でないとき、エージェントは `reason` を `permission` とし、拒否された操作をコメントに書かなければならない（MUST）。

#### Scenario: 権限の拒否で止まった
- **WHEN** 実行が `waiting` を宣言し、resultの `permission_denials` にBashのコマンドが1つある
- **THEN** コメントに質問と拒否されたコマンドが書かれ、マーカーの `reason` が `permission` になる

### Requirement: マーカー
エージェントがissueに書くコメントは、末尾の単独の行にマーカーを持たなければならない（MUST）。
マーカーは `<!-- switchboard v=1 ... -->` の形で、`host`、`session`、`run`、`state`、`reason`、`ack`、`at` を持つ。
`ack` は、その実行がプロンプトに取り込んだ最後のコメントのIDとする。

#### Scenario: 待ちのマーカー
- **WHEN** issue 12の実行が質問を残して終わる
- **THEN** コメントの最後の行に、`state=waiting` と、取り込んだ最後のコメントIDを `ack` に持つマーカーがある

### Requirement: 二重投稿の防止
同じ実行の結果を書き戻すとき、エージェントは、同じ `run` を持つマーカーのコメントが既にあれば、もう一度投稿してはならない（MUST NOT）。

#### Scenario: 書き戻しの途中で止まった
- **WHEN** コメントの投稿の後、ラベルの付け替えの前にエージェントが止まり、再起動する
- **THEN** エージェントはコメントを投稿し直さず、ラベルの付け替えだけを行う。コメントは1件のままである

### Requirement: 完了の検証
実行が `done` を宣言し、プロジェクトに `verify_command` があるとき、エージェントはworktreeでそのコマンドを実行しなければならない（MUST）。
コマンドが失敗したとき、エージェントはpushせず、issueを `waiting(reason=verify)` にしなければならない（MUST）。
`verify_command` が空のとき、エージェントは検証を省き、PRの本文に検証が無いことを書かなければならない（MUST）。

#### Scenario: 検証が失敗する
- **WHEN** 実行が `done` を宣言し、`verify_command` が失敗する
- **THEN** pushもPRの作成もされず、issueは待ちになり、失敗の出力の末尾がコメントに書かれる

### Requirement: push前の出力検査
pushの前に、エージェントは次の4点を確かめなければならない（MUST）。
1点でも満たさないとき、エージェントはpushせず、issueを `waiting(reason=verify)` にしなければならない（MUST）。

- ブランチが `switchboard/issue-<n>` で、`base_branch` より先にコミットがある
- 差分が保護パス（`.github/workflows/`、`.claude/`、`.mcp.json`、`CLAUDE.md`、`AGENTS.md` と `protected_paths_extra`）に触れていない
- 差分、コミットメッセージ、投稿する文章に、トークンや秘密鍵の形式の文字列が無い
- 完了の検証を通っている

#### Scenario: ワークフローの変更
- **WHEN** 実行が `done` を宣言し、差分が `.github/workflows/ci.yml` を変えている
- **THEN** pushされず、issueは待ちになり、保護パスに触れたことがコメントに書かれる

#### Scenario: 秘密情報の混入
- **WHEN** 差分に `ghp_` で始まる文字列がある
- **THEN** pushされず、issueは待ちになる。コメントには該当するファイルの名前が書かれ、文字列そのものは書かれない

### Requirement: pushとPRの作成
出力検査を通ったとき、エージェントは `switchboard/issue-<n>` ブランチだけをpushし、`base_branch` へのdraftのPRを作らなければならない（MUST）。
エージェントは、`base_branch` へpushしてはならない（MUST NOT）。
PRの本文は、issueを閉じる参照（`Closes #<n>`）と、実行の要約と、検証の結果を含まなければならない（MUST）。

#### Scenario: 完了する
- **WHEN** 実行が `done` を宣言し、検証と出力検査を通る
- **THEN** ブランチがpushされ、draftのPRができ、issueにPRのURLのコメントが付いて `agent:done` になる

### Requirement: コメントの言語と検査
エージェントは、設定 `comments` の `language` の言語で、コメントの定型の部分を書かなければならない（MUST）。
`check_command` が空でないとき、エージェントは、投稿する前にコメントのファイルをそのコマンドで検査しなければならない（MUST）。
検査が失敗しても、エージェントはコメントを投稿し、失敗を監査ログに記録しなければならない（MUST）。

#### Scenario: 日本語の設定
- **WHEN** `language: ja` の設定で実行が待ちになる
- **THEN** コメントの見出しと定型文は日本語で書かれる
