# Proposal

## Why

1件の画面は、開くと常にissueの本文から表示する。待ちのissueでは、返答に要るエージェントの質問がスレッドの下にあり、スクロールして探す必要がある。スクロールすると、参照、タイトル、状態の行が画面の外に出て、どのissueを読んでいるのか分からなくなる（#67）。

## What Changes

- 待ちのissueを開いた直後に、エージェントの最新の質問のカードへスクロールする。完了のissueでは、最新の完了の報告のカードへスクロールする
- スクロールするのは開いた直後の1回だけにする。開いている間の読み直しで表示するカードが増えたら、画面の下に「自動更新がありました」の知らせを出し、押すとそのカードへスクロールする
- 「← 一覧へ」、参照、タイトル、状態の行を上に固定する。スクロールしている間は、タイトルを2行で切る

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- `web`: 1件の表示の位置と固定の見出し

## Impact

- `web/ui/src/components/IssuePane.tsx`、`Thread.tsx`、`Header.tsx`、`pages/InboxLayout.tsx`
- `web/ui/src/lib/scrollTarget.ts`（新規）、`web/ui/src/test-setup.ts`
- `DESIGN.md` のLayoutとComponents
