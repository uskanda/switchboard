# Tasks

## 1. 受信箱のサービスの取得

- [x] 1.1 `internal/inbox/speed_test.go` に、先に失敗するテストを書く。エージェントのアカウントのIDを1度だけ引くことと、引けないときは次に引き直すことを確かめる
- [x] 1.2 `internal/inbox/inbox.go` で、エージェントのアカウントのIDを覚えて使い回し、`Detail` でスレッドと同時に引く。1.1のテストが通ることを確かめる

## 2. フォージのアダプタ

- [x] 2.1 GitHubとGitLabのアダプタのテストに、`TestThreadReadsItsPartsAtOnce` を書く。issue、コメント、編集した人の3つの問い合わせが同時に届くことを確かめる
- [x] 2.2 `github.go` と `gitlab.go` の `Thread` で、3つの問い合わせを同時に送る。2.1のテストが通ることを確かめる

## 3. WUIの読み込みの表示

- [x] 3.1 `web/ui/src/pages/issueLoading.test.tsx` に、先に失敗するテストを書く。次の4つを確かめる
  - 初めて開くと、読み込み中の表示が出る
  - 前回の内容を出しながら、更新を取得中であることを示す
  - 別の行から戻ると、すぐに前回の内容に替わる
  - 読み直しに失敗しても、前回の内容が残る
- [x] 3.2 `web/ui/src/lib/issueCache.ts` に、メモリだけに置く上限50件の前回の内容を作る。`web/ui/src/components/ui/spinner.tsx` に回るアイコンを作る
- [x] 3.3 `IssuePane.tsx` で、読み込み中、「更新を取得中」、読み直しの失敗を表示する。3.1のテストが通ることを確かめる
- [x] 3.4 main specsの `web` に「1件の読み込みの表示」の要件を、`inbox-api` の「1件の詳細」に2つの文を足す（PR #45で直接書き換えた）

## 4. 通しの確認

- [x] 4.1 `make verify` が通ることを確かめる（PR #45の「検証」の節と、CIの `verify`）
- [ ] 4.2 GitHubとGitLabの実際のissueで、1件を開くまでの時間が縮んだことを確かめる（PR #45では測っていない）
