# Design

## Context

確認だけの問いの見出しは3か所にある。エージェントがコメントに書く文言（`internal/agent/messages.go` の `planChecks`。`planrun.go` が `#### ` を付けて書く）、Web表示がコメントの見出しの行を探す正規表現（`web/ui/src/lib/planQuestions.ts` の `CHECKS`）、Web表示の節の見出し（`web/ui/src/components/PlanQuestions.tsx` の `checks`）である。正規表現は、コメントの言語と画面の言語が違ってもよいように、日本語と英語の両方を読む。

見出しの要件は、まだアーカイブしていない `weigh-plan-questions` の差分の spec にだけある。

## Goals / Non-Goals

**Goals:**
- 3か所の文言を、日本語「推奨でよいか確認してほしい問い」、英語 "Please check the recommended answers" にそろえる

**Non-Goals:**
- まとめたチップの「推奨どおり 〈件数〉件」、問いの状態の名前（推奨どおり／推奨と違う…）は変えない
- 確認だけの問いの並べ方、節の作り、チップの動きは変えない

## Decisions

- **正規表現は新しい見出しだけを読む**（grilling Q3）。`CHECKS` を `/^#{1,6}\s+(?:推奨でよいか確認してほしい問い|Please check the recommended answers)\s*$/u` に変える。古い見出しも読む案は採らなかった。古い見出しの質問のコメントでは、確認だけの問いが判断が要る問いのカードとして並ぶだけで、答えて送ることはできる
- **差分の spec を作らない**。要件が `openspec/specs/` にまだ無いので、MODIFIED で上書きする先が無い。`weigh-plan-questions` の差分の spec の文言を直接直し、この change は `skip_specs: true` にする。`weigh-plan-questions` をアーカイブすると、新しい文言が `openspec/specs/` に入る
- **文書**（grilling Q4）。今の仕様を書いた文書は新しい文言に直す。決定の記録（`weigh-plan-questions/grilling.md`、docs/DECISIONS.md の既存の項）は書き換えず、DECISIONS.md に新しい項を足す

## Risks / Trade-offs

- [古い見出しで答えを待っている質問のコメントが、確認だけの問いもカードで並べる] → 答えて送ることはできるので、不便は一時的である。新しいラウンドからは新しい見出しになる
