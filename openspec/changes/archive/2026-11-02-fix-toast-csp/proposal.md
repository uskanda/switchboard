# Proposal

## Why

WUIのトーストが画面に出ず、マージの失敗などの知らせが見えていなかった。トーストのライブラリsonnerは、配置のスタイルを実行時に `<style>` で差し込む。ページのCSP（`default-src 'self'`）がこの差し込みを止めたため、トーストは配置されないままページの下に置かれていた。

## What Changes

- `web/ui/src/components/ui/toaster.tsx` で `sonner/dist/styles.css` をimportし、トーストの配置のスタイルをビルドしたCSSに含める
- CSPは緩めない
- ビルドしたCSSに `[data-sonner-toaster]` の規則があることを確かめるテストを足す

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- なし。webの「トースト」の要件（右下に表示する）が満たされるように直す修正で、要件は変えない。そのため `.openspec.yaml` に `skip_specs: true` を置く

## Impact

- `web/ui/src/components/ui/toaster.tsx`
- `web/internal/assets/assets_test.go`
