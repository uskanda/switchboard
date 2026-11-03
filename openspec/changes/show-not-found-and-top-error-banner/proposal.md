# Proposal

## Why

WUIの使い勝手の監査で、2つの問題が見つかった（#89）。

- `/foo/bar` のように、どのルートにも当たらないURLを開くと、ヘッダも出ない空白の画面になる
- 受信箱のサービスに届かない誤りと、フォージを読めなかった誤りが、左の一覧のペインにしか出ない。スマホで1件の画面を見ているときは一覧が隠れるため、誤りが見えない。DESIGN.mdは、誤りのバナーを画面の上に出すと定めている

## What Changes

- どのルートにも当たらないURLでは、ヘッダの付いた「ページがありません」の画面を出し、一覧（`/`）へのリンクを置く。タブのタイトルは「ページがありません - SwitchBoard Inbox」にする
- 受信箱のサービスに届かない誤りと、フォージを読めなかった誤りを、一覧と1件の画面のヘッダの下に、両方のペインにまたがる全幅で出す。一覧のペインには出さない
- 設定の画面とページがありませんの画面には、受信箱の誤りのバナーを出さない。設定の画面は、今のとおり各節の中に誤りを出す

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- `web`: 画面

## Impact

- `web/ui/src/App.tsx`
- `web/ui/src/pages/NotFoundPage.tsx`（新規）
- `web/ui/src/components/InboxErrors.tsx`（新規）
- `web/ui/src/components/InboxList.tsx`
- `web/ui/src/pages/InboxLayout.tsx`
- `DESIGN.md` の誤りのバナーの項とComponentsの項
- Goのサーバーは変えない。未知のパスにもindex.htmlを返している
