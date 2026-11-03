# Proposal

## Why

WUIにはファビコンが無く、背面のタブでは待ちに気付きにくい。スマホのホーム画面に追加するためのmanifestとアイコンも無い。また、一覧の時点を出していないため、自動更新が切れている間に一覧がどれだけ古いかが分からない。

## What Changes

- 通常のファビコンと、待ちの印の付いたファビコンを用意する。待ちのissueが1件以上ある間は、待ちの印のほうを出す
- `manifest.json`と、180px、192px、512pxのPNGのアイコンを配る。`index.html`にmanifest、apple-touch-icon、テーマの色を書く
- 自動更新がliveでない間、ヘッダに「HH:MM時点の一覧」を出す。時刻は`fetched_at`と、最後にliveだった時刻の遅いほうとする

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

- `web`: ファビコン、ホーム画面への追加、一覧の時点の表示を足す

## Impact

- `web/ui/public/`（新規）のファビコン：`favicon.svg`、`favicon-waiting.svg`
- `web/ui/public/`のホーム画面のための素材：`manifest.json`、`apple-touch-icon.png`、`icon-192.png`、`icon-512.png`
- `web/ui/index.html`
- `web/ui/src/lib/`の`title.ts`、`inbox.tsx`、`freshness.ts`（新規）
- `web/ui/src/components/Header.tsx`
- `web/internal/server/server_test.go`（静的なファイルの型を確かめる試験）
- CSP、Goの静的配信のコード、受信箱のサービス、依存のパッケージは変えない
