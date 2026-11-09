# Tasks

## 1. コメントの見出し

- [ ] 1.1 `internal/agent/planrun_test.go` の見出しの文字列を新しい文言に直し、`go test ./internal/agent/` が失敗することを確かめる
- [ ] 1.2 `internal/agent/messages.go` の `planChecks` を、日本語「推奨でよいか確認してほしい問い」、英語 "Please check the recommended answers" に変え、`go test ./internal/agent/` が通ることを確かめる

## 2. Web表示の見出し

- [ ] 2.1 `web/ui/src/lib/planQuestions.test.ts` と `web/ui/src/pages/planChecks.test.tsx` の見出しの文字列を新しい文言に直し、古い見出し「推奨どおりで進めるもの」のコメントでは `check` が付かないことを確かめるテストを足し、Web表示のテストが失敗することを確かめる
- [ ] 2.2 `web/ui/src/lib/planQuestions.ts` の `CHECKS` を新しい見出しだけを読むように変え、`web/ui/src/components/PlanQuestions.tsx` の節の見出しを `推奨でよいか確認してほしい問い（n件）`、`Please check the recommended answers (n)` に変え、Web表示のテストが通ることを確かめる

## 3. 文書

- [ ] 3.1 DESIGN.md、docs/GUIDE.ja.md、docs/GUIDE.md、`openspec/changes/weigh-plan-questions/` の spec（plan-mode と web）、design.md、proposal.md、tasks.md の文言を新しい見出しに直す
- [ ] 3.2 docs/DECISIONS.md に、見出しを依頼の形に変え、古い見出しは読まなくした決定を1項足す
- [ ] 3.3 `推奨どおりで進めるもの` と `Going ahead as recommended` を検索し、残っているのが `weigh-plan-questions/grilling.md`、DECISIONS.md の既存の項、古い見出しを読まないことを確かめるテスト、この change の記録だけであることを確かめる

## 4. 確かめ

- [ ] 4.1 `make verify` が通り、`openspec validate reword-plan-checks-heading --strict` が通ることを確かめる
