# run-outcome Specification

## Purpose

`claude -p` の実行が終わったときに結果を判定し、待ちか完了かをフォージに書き戻す。フォージへの書き込みは、すべてエージェントが行う。

## Requirements

### Requirement: 終了の宣言
実装の実行では、エージェントは、`--json-schema` で終了の宣言のスキーマを渡して `claude -p` を起動しなければならない（MUST）。
宣言は、`status`（`waiting` か `done`）と `summary` を必ず持ち、`waiting` のときは `question` も持つ。
計画の実行の終了の宣言は、`plan-mode` で決める。

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
実装の実行で、resultの `permission_denials` が空でないとき、エージェントは `reason` を `permission` にしなければならない（MUST）。拒否された操作はコメントに書く。
計画の実行で、resultの `permission_denials` が空でないとき、エージェントは `reason` を `permission` にしてはならない（MUST NOT）。
`reason` は `plan-questions` か `plan` のままとし、拒否された操作は、コメントの末尾の折りたたみの節に一覧で書く。

#### Scenario: 権限の拒否で止まった
- **WHEN** 実行が `waiting` を宣言し、resultの `permission_denials` にBashのコマンドが1つある
- **THEN** コメントに質問と拒否されたコマンドが書かれ、マーカーの `reason` が `permission` になる

#### Scenario: 計画の実行で権限が拒否された
- **WHEN** 計画の実行が質問を宣言して終わり、resultの `permission_denials` に `go test ./...` がある
- **THEN** マーカーの `reason` は `plan-questions` で、`go test ./...` はコメントの末尾の `<details>` の中に書かれる

### Requirement: マーカー
エージェントがissueに書くコメントは、末尾の単独の行にマーカーを持たなければならない（MUST）。
マーカーは `<!-- switchboard v=1 ... -->` の形で、`host`、`session`、`run`、`state`、`reason`、`ack`、`at` を持つ。
`ack` は、その実行がプロンプトに取り込んだ最後のコメントのIDとする。
始めるラベルで着手したときの着手のコメントのマーカーは、`mode`（`plan` か `direct`）も持つ。質問のコメント（`reason=plan-questions`）のマーカーは、そのコメントの最後の質問の番号の `q` も持つ。
変更要求のURLを書いた完了のコメントのマーカーは、pushしたコミットのSHAの `sha` と、変更要求の番号の `cr` も持つ。
直す実行の着手のコメントのマーカーは、`reason=fix` を持つ。
マーカーを読むとき、エージェントは、知らない項目を読み飛ばさなければならない（MUST）。

#### Scenario: 待ちのマーカー
- **WHEN** issue 12の実行が質問を残して終わる
- **THEN** コメントの最後の行に、`state=waiting` と、取り込んだ最後のコメントIDを `ack` に持つマーカーがある

#### Scenario: 計画の質問のマーカー
- **WHEN** 計画の実行がQ4とQ5を聞くコメントを投稿する
- **THEN** マーカーに `reason=plan-questions` と `q=5` がある

#### Scenario: 完了のマーカー
- **WHEN** issue 12の実行が、コミット `abc123` をpushしてPR #48を作って完了する
- **THEN** 完了のコメントのマーカーに `state=done`、`sha=abc123`、`cr=48` がある

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
- 差分が保護パスに触れていない
- 差分、コミットメッセージ、投稿する文章に、トークンや秘密鍵の形式の文字列が無い
- 完了の検証を通っている

保護パスの既定は、`.github/workflows/`、`.gitlab-ci.yml`、`.claude/`、`.mcp.json`、`CLAUDE.md`、`AGENTS.md` とする。
`protected_paths_extra` に書いたパスも、保護パスに加える。
プロジェクトのフォージにかかわらず、同じ保護パスを使う。

#### Scenario: ワークフローの変更
- **WHEN** 実行が `done` を宣言し、差分が `.github/workflows/ci.yml` を変えている
- **THEN** pushされず、issueは待ちになり、保護パスに触れたことがコメントに書かれる

#### Scenario: GitLabのCI定義の変更
- **WHEN** GitLabのプロジェクトで、実行が `done` を宣言し、差分が `.gitlab-ci.yml` を変えている
- **THEN** pushされず、issueは待ちになり、保護パスに触れたことがコメントに書かれる

#### Scenario: 秘密情報の混入
- **WHEN** 差分に `ghp_` で始まる文字列がある
- **THEN** pushされず、issueは待ちになる。コメントには該当するファイルの名前が書かれ、文字列そのものは書かれない

### Requirement: pushとPRの作成
出力検査を通ったとき、エージェントは `switchboard/issue-<n>` ブランチだけをpushし、`base_branch` へのdraftの変更要求を作らなければならない（MUST）。
変更要求は、GitHubではPR、GitLabではMRとする。
エージェントは、`base_branch` へpushしてはならない（MUST NOT）。
変更要求の本文は、issueを閉じる参照（`Closes #<n>`）と、実行の要約と、検証の結果を含まなければならない（MUST）。
`switchboard/issue-<n>` から `base_branch` への開いた変更要求が既にあるとき、エージェントは新しい変更要求を作ってはならない（MUST NOT）。
そのとき、エージェントは、pushの後に、既存の変更要求のURLをissueにコメントして `agent:done` にしなければならない（MUST）。

#### Scenario: 完了する
- **WHEN** 実行が `done` を宣言し、検証と出力検査を通る
- **THEN** ブランチがpushされ、draftのPRができ、issueにPRのURLのコメントが付いて `agent:done` になる

#### Scenario: GitLabで完了する
- **WHEN** GitLabのプロジェクトで、実行が `done` を宣言し、検証と出力検査を通る
- **THEN** ブランチがpushされ、draftのMRができ、issueにMRのURLのコメントが付いて `agent:done` になる

#### Scenario: 開いたPRがあるブランチで再実行して完了する
- **WHEN** `switchboard/issue-34` から `main` への開いたPR #48があるissue 34で、再実行が `done` を宣言し、検証と出力検査を通る
- **THEN** ブランチがpushされ、PRは新しく作られず、issueにPR #48のURLのコメントが付いて `agent:done` になる

#### Scenario: 閉じたPRだけがあるブランチで完了する
- **WHEN** `switchboard/issue-34` のPRが閉じられていて、開いたPRが無いissue 34で、実行が `done` を宣言し、検証と出力検査を通る
- **THEN** 新しいdraftのPRができ、issueにそのURLのコメントが付く

### Requirement: コメントの言語と検査
エージェントは、設定 `comments` の `language` の言語で、コメントの定型の部分を書かなければならない（MUST）。
`check_command` が空でないとき、エージェントは、投稿する前にコメントのファイルをそのコマンドで検査しなければならない（MUST）。
検査が失敗しても、エージェントはコメントを投稿し、失敗を監査ログに記録しなければならない（MUST）。

#### Scenario: 日本語の設定
- **WHEN** `language: ja` の設定で実行が待ちになる
- **THEN** コメントの見出しと定型文は日本語で書かれる

### Requirement: 変更要求の作成の失敗の理由
変更要求の作成が失敗したとき、エージェントは、フォージが応答で返した失敗の理由をissueのコメントに書かなければならない（MUST）。
GitHubでは、応答の `message` に加えて `errors` の各項目の内容を書く。

#### Scenario: GitHubが422を返す
- **WHEN** PRの作成に、GitHubが422を返す。`message` は `Validation Failed` で、`errors` の項目は `No commits between X and Y` である
- **THEN** issueは `waiting(reason=error)` になる。コメントには `Validation Failed` と `No commits between X and Y` が書かれる

### Requirement: pushの失敗の分類
pushが失敗したとき、エージェントは、gitの出力から、失敗を次の4つのどれかに分けなければならない（MUST）。
4つとは、一時的な失敗、non-fast-forward、権限・認証の失敗、リモートの規則による拒否である。
一時的な失敗は、ネットワークの誤り、5xx、タイムアウトをいう。リモートの規則による拒否は、ブランチの保護、pre-receive hook、push rule、サイズの上限による拒否をいう。
どれにも当てはまらない失敗は、リモートの規則による拒否として扱う。

#### Scenario: リモートの枝が先に進んでいる
- **WHEN** pushに対して、gitが `! [rejected] ... (fetch first)` を出力する
- **THEN** 失敗はnon-fast-forwardに分けられる

#### Scenario: 保護されたブランチ
- **WHEN** pushに対して、GitHubが `GH006: Protected branch update failed` を返す
- **THEN** 失敗はリモートの規則による拒否に分けられる

### Requirement: 一時的なpushの失敗
pushが一時的な失敗のとき、エージェントは、同じ実行の中で、間隔を空けてpushをやり直さなければならない（MUST）。pushは最初の1回を含めて3回まで試みる。
3回とも失敗したとき、エージェントはissueを `waiting(reason=error)` にしなければならない（MUST）。

#### Scenario: 1回だけ接続が切れる
- **WHEN** 最初のpushが接続の誤りで失敗し、2回目のpushが成功する
- **THEN** 変更要求が作られ、issueは `agent:done` になる。pushの失敗はコメントに書かれない

### Requirement: non-fast-forwardのpushの失敗
pushがnon-fast-forwardで失敗し、直した回数が `<agent.max_fix_attempts>` 未満のとき、エージェントは、リモートの枝をfetchしなければならない（MUST）。リモートの枝とは、originの `switchboard/issue-<n>` をいう。
そのうえで、エージェントは、issueを `agent:running` のまま、直す実行を始めなければならない（MUST）。その着手のコメントには、pushが拒否されたことと、何回目の直しかを書く。
直した回数が `<agent.max_fix_attempts>` に達しているとき、エージェントはissueを `waiting(reason=push)` にしなければならない（MUST）。
エージェントは、強制pushをしてはならない（MUST NOT）。

#### Scenario: 人が同じブランチにコミットを足していた
- **WHEN** 人が `switchboard/issue-12` にコミットを足した後で、issue 12の再実行が完了し、pushがnon-fast-forwardで拒否される
- **THEN** エージェントはリモートの枝をfetchし、`reason=fix` のマーカーの着手のコメントを付けて、直す実行を始める。強制pushはされない

### Requirement: 権限と規則によるpushの失敗
pushが権限・認証の失敗か、リモートの規則による拒否のとき、エージェントは、直す実行を始めずに、issueを `waiting(reason=push)` にしなければならない（MUST）。
そのコメントには、失敗の分類と、gitの出力と、人がすべきことを書かなければならない（MUST）。権限・認証の失敗では、トークンの権限を確かめるよう書く。リモートの規則による拒否では、Claudeに直させたいときは返信すればよいことを書く。

#### Scenario: 書き込みの権限が無いトークン
- **WHEN** pushに対して、GitHubが403を返す
- **THEN** issueは `waiting(reason=push)` になり、コメントには権限・認証の失敗であることと、トークンのContentsの権限を確かめるよう書かれる

### Requirement: pushの出力の伏せ字
pushの失敗をコメントやプロンプトに書くとき、エージェントは、gitの出力の中のトークンや秘密鍵の形式の文字列を伏せ字にしなければならない（MUST）。

#### Scenario: URLにトークンが含まれる
- **WHEN** pushの失敗の出力に、トークンを含むリモートのURLが含まれる
- **THEN** コメントでは、そのトークンが伏せ字になっている

### Requirement: 自動で直す回数
エージェントは、CIの失敗、non-fast-forward、変更要求の衝突のどれで始めた直す実行も、1つのissueの直した回数として合わせて数えなければならない（MUST）。
直した回数は、最後のリセットより後の、`reason=fix` のマーカーを持つエージェントの着手のコメントの数とする。
信頼済みの人のコメントか始めるラベルで実行が始まったとき、その時点でリセットする。

#### Scenario: 人の返信で数え直す
- **WHEN** 2回の直しの後で `waiting(reason=ci)` になったissueへマスターが返信する。その実行が完了した後で、CIがまた失敗する
- **THEN** 直す実行が始まり、着手のコメントは「（1/2）」と書く

#### Scenario: CIの直しの後で衝突する
- **WHEN** `<agent.max_fix_attempts>` が2の設定で、CIの失敗を1回直して完了した後で、PRがbaseと衝突する
- **THEN** 衝突を直す実行が始まり、着手のコメントは「（2/2）」と書く

### Requirement: 直す実行の終わり
直す実行の終わりを、エージェントは、ほかの実装の実行と同じく扱わなければならない（MUST）。
完了の検証と、push前の出力検査を行う。完了したら、同じブランチにpushし、開いた変更要求を再び使う。

#### Scenario: 直して完了する
- **WHEN** CIの失敗を直す実行が `done` を宣言し、検証と出力検査を通る
- **THEN** 同じPRに新しいコミットが載り、issueには、新しい `sha` を持つ完了のコメントが付いて `agent:done` になる

#### Scenario: ワークフローを変えないと直せない
- **WHEN** 直す実行が `.github/workflows/ci.yml` を変えて `done` を宣言する
- **THEN** pushされず、issueは `waiting(reason=verify)` になる

### Requirement: 書き戻しの再試行
実行の結果の書き戻しが失敗したとき、エージェントは、書き戻せるまでポーリングごとに再試行しなければならない（MUST）。
再試行の回数に上限は設けない。再試行は、エージェントを再起動しなくても行う。

#### Scenario: ラベルの付け替えだけが失敗した
- **WHEN** 実行が質問を残して終わり、質問のコメントの投稿は成功し、`agent:waiting` への付け替えが1回失敗する
- **THEN** 次のポーリングでissueは `agent:waiting` になり、質問のコメントは1件のままである

#### Scenario: コメントを投稿する前に失敗した
- **WHEN** 実行が質問を残して終わり、書き戻しの最初のissueの取得がフォージの502で失敗する
- **THEN** 次のポーリングで質問のコメントが投稿され、issueは `agent:waiting` になる

### Requirement: 再試行でラベルを変えない場合
書き戻しの再試行のとき、issueが次のどれかに当たれば、エージェントはラベルを変えてはならない（MUST NOT）。

- openでない
- ラベルが `agent:running` でない
- 最新の着手のコメントが、その実行のものでない

このとき、結果のコメントがまだ投稿されていなければ、エージェントはコメントだけを投稿し、その実行の再試行をやめる。

#### Scenario: 再試行の前にissueが閉じられた
- **WHEN** 完了した実行の書き戻しでコメントの投稿が失敗し、次のポーリングの前に、マスターがissueをcloseする
- **THEN** 次のポーリングで完了のコメントが投稿され、issueのラベルは変わらない。そのあと再試行は行われない

#### Scenario: 再試行の前に人がラベルを変えた
- **WHEN** 書き戻しのラベルの付け替えが失敗し、次のポーリングの前に、マスターが `agent:running` を外して `agent:ready` を付ける
- **THEN** 結果のコメントは1件のままで、ラベルはマスターが変えたままである。issueは `agent:ready` として評価される
