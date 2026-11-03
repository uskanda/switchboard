# Proposal

## Why

計画の質問が何ラウンドも続くと、1件の表示のスレッドがとても長くなる。スマホで#70を開いたときは、文書の高さが10,000pxを超えた。答え終わった古い質問が並ぶため、まだ答える必要があるように見える（#85）。

## What Changes

- エージェントの最新の待ちより前の待ちのコメントと、そのあとに続く人のコメントを1ラウンドとし、1行に畳む。押すと開ける
- 畳んだ1行には、エージェントの印、「答えた質問」、返答の件数、質問の日時を出す
- 開いた状態は、同じissueの読み直しでは保ち、別のissueへ移ると畳み直す
- 開いた直後のスクロールや知らせの行き先が畳んだラウンドの中にあるときは、そのラウンドを開いてからスクロールする

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- `web`: スレッドの表示、1件の表示の位置

## Impact

- `web/ui/src/lib/scrollTarget.ts`：スレッドの並びを返す関数`threadOf`
- `web/ui/src/components/Thread.tsx`：ラウンドを `<details>` で描く
- `web/ui/src/components/IssuePane.tsx`：`<Thread>`のkey、畳んだラウンドを開いてからのスクロール
