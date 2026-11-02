# Proposal

## Why

WUIのヘッダの名前は「受信箱」で、どの道具の画面かが名前から分からなかった（#62）。

## What Changes

- ヘッダの名前を「SwitchBoard Inbox」にする
- 幅640px未満の画面では、名前を「SwitchBoard」に縮める
- ブラウザのタブのタイトルは変えない。issueの対象はヘッダだけである

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- `web`: ヘッダの名前

## Impact

- `web/ui/src/components/Header.tsx`
- `web/ui/src/pages/pages.test.tsx`
- `DESIGN.md` のComponentsのヘッダの記述
