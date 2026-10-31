# Proposal

## Why

issueが `agent:running` の間、実行が止まっているのか、まだ処理しているのかを判断できない。受信箱は待ちのissueだけを出し、実行中のissueはどこにも出ない。`gh:uskanda/switchboard` #19では、着手のあと数分の間、Web表示には何も出ず、動いているかどうかが分からなかった（issue #21）。

## What Changes

- エージェントは、実行ごとに進捗のコメントを1つ投稿し、1分ごとに書き換えて1行ずつ足す
  - 1行には、時刻、経過時間、使ったツールの名前と回数、何を目的として作業しているかの1行を書く
  - 目的の1行は、`claude -p` の最新のassistantのテキストから取る。前の行と同じ内容なら足さない
  - 書き換えるたびに、秘密の検査と `comments.check_command` を通す
  - 結果のコメントを投稿したら、進捗のコメントを消す。エージェントが落ちて残ったものは、再起動のあとの回復で消す
- フォージのクライアントに、コメントの書き換えを足す
- 受信箱（`switchboard inbox`、`inboxapi`、Web表示）に、実行中のissueを待ちとは別の欄で出す。実行中の行には、端末の名前、経過時間、最新の進捗の1行を出す。実行中のissueには返答させない

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- `agent-run`: 実行中の進捗のコメント
- `inbox`: 実行中のissueの一覧と、JSONの状態の項目
- `inbox-api`: 実行中のissueの一覧と詳細
- `web`: 実行中のissueの表示

## Impact

- `internal/forge`（GitHub、GitLab、偽物）に、コメントの書き換えを足す
- `internal/claudecode` か `internal/agent` で、実行中の `stream.jsonl` から、ツールの回数と最新のassistantのテキストを読む
- `internal/agent` の実行と回復に、進捗のコメントの投稿、書き換え、削除を足す
- `internal/marker` に、進捗のコメントの状態（`progress`）を足す
- `internal/inbox`、`inboxapi`、`internal/inboxserve`、`web/` に、実行中のissueを足す
- GitHubの受信箱は、待ちと実行中を1回の検索で取ることを目指す。GitHubの検索でラベルのOR（`label:"agent:waiting","agent:running"`）が使えるかは、実物で確かめる（V28）
- README.mdを更新する
