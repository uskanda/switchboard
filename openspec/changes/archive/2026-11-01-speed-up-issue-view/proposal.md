# Proposal

## Why

WUIでissueを開くと、内容が出るまで数秒のあいだ空の表示が続いていた（#44）。読み込み中かどうかが画面から分からず、前に開いたissueでも毎回フォージの応答を待っていた。

## What Changes

- 初めて開くissueでは、回るアイコンと「読み込んでいます」を表示する
- 同じタブで前に読んだissueは、前回の内容をすぐに表示し、読み直しが届くまで「更新を取得中」と示す。前回の内容はメモリにだけ置き、上限は50件とする
- 読み直しに失敗したときは、前回の内容を残し、前回のものを表示していることを示す
- 受信箱のサービスは、エージェントのアカウントのIDを1度引いたら使い回す。引けなかったときは覚えない
- GitHubとGitLabのアダプタは、issue、コメント、編集した人の3つの問い合わせを同時に送る

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- `inbox-api`: 1件の詳細に、エージェントのアカウントのIDの使い回しと、3つの問い合わせを同時に送ることを足す
- `web`: 1件の読み込みの表示の要件を足す

## Impact

- `internal/inbox/inbox.go`、`internal/inbox/speed_test.go`
- `internal/forge/github/github.go`、`internal/forge/gitlab/gitlab.go` とそれぞれのテスト
- `web/ui/src/components/IssuePane.tsx`
- `web/ui/src/components/ui/spinner.tsx`、`web/ui/src/lib/issueCache.ts`
- `web/ui/src/pages/issueLoading.test.tsx`、`web/ui/src/test-setup.ts`
- main specsの `inbox-api` と `web`
  - PR #45は、変更を作らずにこの2つを直接書き換えた
  - この変更の仕様の差分は、その書き換えを事後に記録したものである
