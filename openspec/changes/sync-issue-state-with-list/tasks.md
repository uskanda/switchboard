# Tasks

## 1. 受信箱のサービス

- [ ] 1.1 `internal/inboxserve`に、配った一覧と詳細の状態が食い違うときに読み直しが頼まれることのテストを書き、落ちることを確かめる。状態が違う、同じ、行が無い、配った一覧が無い、の4つを確かめる
- [ ] 1.2 `Server.issue`で詳細を読んだあと一覧と比べ、食い違えば`readAgain()`を呼ぶ。1.1のテストが通ることを確かめる

## 2. Web UI

- [ ] 2.1 `lib/api.test.ts`に状態の比較の関数のテストを書き、落ちることを確かめる
- [ ] 2.2 `lib/api.ts`に`itemState`、`detailState`、`stateDiffers`を足し、2.1のテストが通ることを確かめる
- [ ] 2.3 `IssuePane`のテストに、知らせが出る、同じ状態なら出ない、読み直しの最中は出ない、一覧が追いつくと消える、を書き、落ちることを確かめる
- [ ] 2.4 `IssuePane`の状態の行に知らせを出し、2.3のテストが通ることを確かめる

## 3. 文書と確認

- [ ] 3.1 `DESIGN.md`の1件の見出しの項に、知らせを足す
- [ ] 3.2 `openspec validate sync-issue-state-with-list --strict`と`make verify`を通す
