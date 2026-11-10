# Proposal

## Why

確認だけの問いの見出し「推奨どおりで進めるもの」（英語 "Going ahead as recommended"）は、エージェントが推奨で進めると宣言しているように読める。使用者が推奨でよいかを確かめる場所だ、という意図が伝わりにくい（issue #197）。

## What Changes

- 質問のコメントで確認だけの問いの前に置く見出しを、「推奨でよいか確認してほしい問い」（英語 "Please check the recommended answers"）に変える
- Web表示の確認だけの問いの節の見出しを、`推奨でよいか確認してほしい問い（n件）`（英語 `Please check the recommended answers (n)`）に変える
- **BREAKING** Web表示は、質問のコメントの見出しとして新しい文言だけを読む。古い見出しで投稿されて答えを待っている質問のコメントでは、確認だけの問いが普通のカードとして並ぶ。答えて送ることはこれまでどおりできる
- まとめたチップの「推奨どおり 3件」は、問いの状態の名前なので変えない
- DESIGN.md、docs/GUIDE.ja.md、docs/GUIDE.md、まだアーカイブしていない weigh-plan-questions の spec、design、proposal を新しい文言に直す。docs/DECISIONS.md に、文言を変えた決定を1項足す

## Capabilities

### New Capabilities

なし

### Modified Capabilities

なし。見出しの文言を定める要件は、まだアーカイブしていない `weigh-plan-questions` の差分の spec（plan-mode と web）にだけあり、`openspec/specs/` にはまだ無い。そのため、この change では差分の spec を作らず、`weigh-plan-questions` の差分の spec の文言を直接直す。`.openspec.yaml` に `skip_specs: true` を置く。

## Impact

- `internal/agent/messages.go`：コメントの文言の表の `planChecks`
- `web/ui/src/components/PlanQuestions.tsx`：節の見出しの文言
- `web/ui/src/lib/planQuestions.ts`：見出しの行を読む正規表現 `CHECKS`
- テスト：`internal/agent/planrun_test.go`、`web/ui/src/lib/planQuestions.test.ts`、`web/ui/src/pages/planChecks.test.tsx`
- 文書：DESIGN.md、docs/GUIDE.ja.md、docs/GUIDE.md、docs/DECISIONS.md、`openspec/changes/weigh-plan-questions/`
