# Tasks

## 1. トーストの配置のスタイル

- [x] 1.1 `web/internal/assets/assets_test.go` に `TestBuiltStylesheetCarriesTheToasterLayout` を書く。ビルドしたCSSに `[data-sonner-toaster]` の規則があることを確かめる
- [x] 1.2 `web/ui/src/components/ui/toaster.tsx` で `sonner/dist/styles.css` をimportし、配置のスタイルをビルドしたCSSに含める。CSPは緩めない。1.1のテストが通ることを確かめる

## 2. 通しの確認

- [x] 2.1 `make verify` が通ることを確かめる（PR #81の説明と、CIの `verify`）
- [x] 2.2 headlessのChromiumで直したWeb表示を開き、`/api/merge` を横取りして誤りを返すと、トーストが右下に出ることを確かめる（PR #81の説明に記録）
- [ ] 2.3 #67・#70・#72で、デプロイしたあとにマージのボタンをもう一度押す。断られた理由がトーストに出ることを確かめる
