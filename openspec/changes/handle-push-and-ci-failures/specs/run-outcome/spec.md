# Spec Delta

## MODIFIED Requirements

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

## ADDED Requirements

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
エージェントは、CIの失敗とnon-fast-forwardのどちらで始めた直す実行も、1つのissueの直した回数として合わせて数えなければならない（MUST）。
直した回数は、最後のリセットより後の、`reason=fix` のマーカーを持つエージェントの着手のコメントの数とする。
信頼済みの人のコメントか始めるラベルで実行が始まったとき、その時点でリセットする。

#### Scenario: 人の返信で数え直す
- **WHEN** 2回の直しの後で `waiting(reason=ci)` になったissueへマスターが返信する。その実行が完了した後で、CIがまた失敗する
- **THEN** 直す実行が始まり、着手のコメントは「（1/2）」と書く

### Requirement: 直す実行の終わり
直す実行の終わりを、エージェントは、ほかの実装の実行と同じく扱わなければならない（MUST）。
完了の検証と、push前の出力検査を行う。完了したら、同じブランチにpushし、開いた変更要求を再び使う。

#### Scenario: 直して完了する
- **WHEN** CIの失敗を直す実行が `done` を宣言し、検証と出力検査を通る
- **THEN** 同じPRに新しいコミットが載り、issueには、新しい `sha` を持つ完了のコメントが付いて `agent:done` になる

#### Scenario: ワークフローを変えないと直せない
- **WHEN** 直す実行が `.github/workflows/ci.yml` を変えて `done` を宣言する
- **THEN** pushされず、issueは `waiting(reason=verify)` になる
