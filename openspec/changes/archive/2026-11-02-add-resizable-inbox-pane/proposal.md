# Proposal

## Why

幅1024px以上の画面では、一覧のペインの幅が360pxに固定されていた。タイトルの長いissueを一覧で読みにくく、広い画面の幅を一覧に使えなかった（#63）。

## What Changes

- 幅1024px以上の2ペインの画面に、一覧と1件の境目を置く。境目をドラッグすると、一覧の幅が280pxから640pxの間で変わる
- 右のペインには常に480px以上を残す。ウィンドウを狭めたときも、一覧の幅はこの範囲に収まるよう縮む
- 境目はキーボードで動かせる。矢印キーで16pxずつ、HomeとEndで端まで動く。ダブルクリックで既定の360pxに戻る
- 変えた幅はブラウザに保存し、次に開いたときも同じ幅で出す
- 幅1024px未満の画面には境目を出さない

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- `web`: 一覧のペインの幅の変更

## Impact

- `web/ui/src/pages/InboxLayout.tsx`
- `web/ui/src/lib/paneWidth.ts`（新規）
- `web/ui/src/lib/paneWidth.test.ts`、`web/ui/src/pages/paneResize.test.tsx`（新規）
- `DESIGN.md` のLayout
