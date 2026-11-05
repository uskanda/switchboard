# Proposal

## Why

計画の待ち（`plan`、`plan-questions`）では、一覧のカードの3行目に、計画の題ではなく途中の箇条書きが出る。そのため、何を承認するのかが一覧から分からない（issue #86）。計画の宣言には題にあたる欄が無いので、題を宣言させ、コメントの冒頭に置く。

## What Changes

- 計画の実行の宣言に`title`（計画の題を1行で）を足し、`questions`と`plan`のどちらでも必須にする。題が無いか空白だけの宣言は、`waiting(reason=error)`として扱う
- 質問のコメントと計画のコメントで、見出しのすぐ次に「**計画：** 〈題〉」（英語では「**Plan:** 〈title〉」）の段落を置く
- 計画の実行への指示に、`title`の書き方（短い1行、Markdownなし、issueの題を繰り返さない）を書く
- 受信箱の一覧と待ちの通知は、質問の冒頭として計画の題を出す。質問の待ちの通知で最初の問いを抜き出していた処理はやめ、題を出す
- 題の無い古いコメントでは、今までどおりの抜粋を出す

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- `plan-mode`：計画の実行の終了の宣言に`title`を足す。質問のコメントと計画のコメントの冒頭に題の段落を置く
- `notify`：計画の待ちの通知の抜粋を、計画の題にする
- `web`：一覧の質問の冒頭に、計画の待ちでは「計画： 〈題〉」が出るシナリオを足す

## Impact

- Go：`internal/agent/planrun.go`（スキーマ、宣言、コメントの書式）、`internal/agent/messages.go`（題のラベル）、`internal/agent/writeback.go`（通知の抜粋）、`internal/prompt/prompt.go`（計画の実行への指示）
- 受信箱（`internal/inbox`）とWeb UI（`InboxList.tsx`）のコードは変えない。テストだけを足す
