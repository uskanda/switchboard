# Proposal

## Why

1日に何度も短く返答する使い方（PRODUCT.md）で、返答を送るたびに一覧へ戻って次の待ちを選び直す必要がある。キーボードでは、一覧の項目から返答の欄までTabを10回ほど押す。Web UIの使い勝手の監査で、重要度は中と見られた（issue #83）。

## What Changes

- 返答を送れたら、「送りました」の行の直後に「次の待ちへ」のボタンを出し、フォーカスをそこへ移す。行き先が無ければ「他に待ちはありません」と出し、フォーカスは今までどおり「送りました」の行へ移す
- 「次の待ち」は、一覧の「返答待ち」の節で今のissueの1つ下から探し、末尾から先頭に戻る。今のissueと、このタブで送ったばかりでまだ同じ待ちに残るissueは飛ばす
- `j`と`k`で「返答待ち」の節の中を移り、そのissueを開く。端で止まる
- `r`で返答の欄にフォーカスを移す
- 幅1024px以上のとき、一覧のペインの下にキーの案内を1行出す

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

- `web`：要件「送ったあとのフォーカス」を変え、要件「次の待ちへ」「次の待ちの行き先」「待ちの間を移るキー」「返答の欄へ移るキー」「キーを受けないとき」「キーの案内」を足す

## Impact

- `web/ui/src/components/IssuePane.tsx`、`web/ui/src/pages/InboxLayout.tsx`
- 新しいモジュール`web/ui/src/lib/sent.ts`と`web/ui/src/lib/waits.ts`
- `DESIGN.md`のLayoutとComponents
- サーバーとAPIは変えない
