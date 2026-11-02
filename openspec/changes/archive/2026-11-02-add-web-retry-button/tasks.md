# Tasks

## 1. 画面（`web/ui`）

- [x] 1.1 `IssuePane.tsx` に `RETRY_TEXT` と `retrying` を足し、理由が `error` の待ちに「再実行する」（主）と「返答を送る」（副）を並べる。`failure.test.tsx` のテストで、空の欄で押すと決まった文だけが `/api/reply` に1回届き「送りました」が出ることを確かめた
- [x] 1.2 `IssuePane.tsx` で、返答の欄に文があるときは「再実行する」を押せなくする。欄に書くと押せなくなり、「返答を送る」で書いた文が送られることを `failure.test.tsx` のテストで確かめた
- [x] 1.3 `IssuePane.tsx` で、待ちでないissueと、ほかの失敗の理由の待ちでは再実行できなくする。`failure.test.tsx` のテストで2つを確かめた。実行中と順番待ちのissueでは「再実行する」が出ないか押せない。理由が `verify`、`permission`、`push`、`ci` の待ちには出ない
- [x] 1.4 `PlanHint` に理由が `error` の案内を1行足す。案内に「再実行する」の使い方が出ることを `failure.test.tsx` のテストで確かめた

## 2. 文書

- [x] 2.1 DESIGN.mdの1件の画面の項に、実行の失敗の待ちの案内とボタンの色を足す。PR #78の差分で確かめた
- [x] 2.2 README.mdのWeb表示の説明に、「再実行する」のボタンを1文足す。PR #78の差分で確かめた
- [ ] 2.3 DESIGN.mdの「計画の質問のカード」の行に残った重複（行の途中から同じ項がもう1度始まる）を直し、`designmd lint DESIGN.md` を通す

## 3. 仕上げ

- [x] 3.1 `make verify` を通す。PR #78のCI（`verify`）が成功した
- [ ] 3.2 `switchboard-web` の実際の画面で、理由が `error` の待ちのissueに「再実行する」を押し、エージェントが再実行を始めることを手で確かめる
