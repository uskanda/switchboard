# Tasks

## 1. 受信箱のAPI

- [x] 1.1 `internal/inboxserve/issues_test.go` に2つのテストを足し、失敗することを確かめる。一覧の最後に `none` が来ること（`labels.plan` が空でも出る）と、種別 `none` でラベルの無いissueができること（GitHubとGitLab）である
- [x] 1.2 `inboxapi.StartNone` を足し、`starts()` の最後に `none` を常に足す。`createIssue` は `none` ならラベルを付けずに作る。`go test ./internal/inboxserve/` が通り、空の種別と `agent:done` が `invalid` のままであることを確かめる

## 2. WUI

- [x] 2.1 `web/ui/src/pages/newIssue.test.tsx` に2つのテストを足し、失敗することを確かめる。最後の選択肢が「ラベルなし（実装開始しない）」でラベル名を出さないことと、選んで作ると `start: "none"` が送られることである
- [x] 2.2 `StartKind` に `none` を足し、`NewIssue.tsx` の文言とラベル名の表示を直す。`vitest` と `tsc` が通ることを確かめる

## 3. 文書

- [x] 3.1 `docs/DECISIONS.md` にADR-048を足し、ADR-040に注記する。`docs/SPEC.md`、`README.md`、`DESIGN.md` の開始ラベルの説明に「ラベルなし」を足し、textlintで確かめる

## 4. 確認

- [x] 4.1 `make verify` と `openspec validate add-unlabeled-new-issue --strict` が通ることを確かめる
