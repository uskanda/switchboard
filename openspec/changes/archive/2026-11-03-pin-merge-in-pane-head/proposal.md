# Proposal

## Why

1件の表示の「マージ」のボタンは、スクロールで流れる本文の先頭にある。長いスレッドを下まで読むと、ボタンは画面の外に出る。読み終えたところでマージしたくても、上まで戻らないと押せない（#127）。

## What Changes

- 1件の表示で、PR/MRのリンク、「マージ」、「自動マージを取り消す」を、固定の見出しの状態の行の下へ移す。スクロールしても上に残る
- 本文の先頭にあったこれらの行はなくす。ボタンは右ペインに1か所だけ置く
- 出す条件と、`?action=merge`でダイアログを開く動きは変えない

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- `web`: 1件の表示の位置と固定の見出し、マージのボタン

## Impact

- `web/ui/src/components/IssuePane.tsx`
- `web/ui/src/pages/merge.test.tsx`
- `DESIGN.md`のComponents（1件の見出し、マージ、自動マージ）
