# Proposal

## Why

一覧のカードの形（幅640px未満と左のペイン）では、完了のPR/MRへのリンクとマージのボタンが、カードの枠の外に並んでいた。完了が何件かあると、どのカードの操作か分かりにくい。Web UIの使いやすさの監査で見つかり、issue #87に分けたものである。

## What Changes

- 完了のカードで、PR/MRへのリンクとマージのボタンを、そのカードの枠の中に置く
- リンクとボタンは、1件へのリンクの中に入れない。枠の中で、1件へのリンクの下に兄弟として並べる
- 1件へのリンクとPR/MRの行の間に、`outline`の1pxの線を引く
- 線より上は、余白も含めて全部が1件へのリンクになる。選択中の`primary`の2pxの枠とフォーカスの枠は、枠の内側に収める
- 表の形（幅640〜1023px）は、もともと同じセルの中にあるので変えない
- DESIGN.mdの一覧の行とマージの項に、この組み方を書く

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- `web`: 完了の行で、PR/MRへのリンクとマージのボタンを置く場所

## Impact

- `web/ui/src/components/InboxList.tsx`（`ChangeLink`、`Items`）
- `web/ui/src/pages/merge.test.tsx`
- `DESIGN.md` のComponents（一覧の行、マージ）
