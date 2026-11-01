# Design

## Context

`finish`（`internal/agent/run.go`）は、pushの後に必ず `CreateChangeRequest` を呼ぶ。`Forge` には、ブランチの変更要求を探す操作が無い。
GitHubのクライアントの `do` は、失敗の応答から `message` だけを読む。GitLabのクライアントは `message` と `error` を詳しく読んでいる。

## Goals / Non-Goals

**Goals:**

- 開いた変更要求があるブランチの再実行を、そのURLで完了にする
- GitHubの失敗の理由をコメントに出す

**Non-Goals:**

- 既存の変更要求の本文やタイトルを、再実行の要約で書き換えること。要約は完了のコメントに残る
- draftを外すことや、CIの結果を見ること。pushやCIのエラーへの対処は別の変更で扱う

## Decisions

### 作る前に探す

`Forge` に `OpenChangeRequest(ctx, owner, repo, head, base string) (string, error)` を足す。開いた変更要求のURLを返し、無ければ `forge.ErrNotFound` を返す。`finish` はpushの後にこれを呼び、見つかればそのURLで完了にし、`ErrNotFound` のときだけ作る。

- GitHub：`GET /repos/{o}/{r}/pulls?state=open&head={o}:{head}&base={base}`。フォークは使わないので、headの所有者はリポジトリの所有者とする
- GitLab：`GET /projects/{id}/merge_requests?state=opened&source_branch={head}&target_branch={base}`

退けた案は、作成が422（GitLabは409）で失敗した後に探す方法。毎回の完了でAPIの呼び出しを1回減らせる。しかし422には、ブランチに差分が無いときなど別の理由もあり、理由の文を解釈して分ける必要がある。フォージの文言に依存するので採らない。

探す操作自体が失敗したとき（`ErrNotFound` 以外）は、作成の失敗と同じく `prFailed`／`mrFailed` で待ちにする。

### GitHubのエラーの詳細

`apiError` に、応答の `errors` の各項目の `message`（無ければ `code` と `field`）を `; ` でつないだ文を持たせる。`Error()` は、その文を `message` の後ろに `: <詳細>` として付ける。項目が無いときの文言は今と同じにする。

## Risks / Trade-offs

- 完了ごとにAPI呼び出しが1回増える → 1回の実行に1回だけで、レート制限に影響しない
- 人が同じブランチから別のPRを開いていた → `switchboard/issue-<n>` はエージェントのブランチなので、そのPRを再利用して困ることはない
