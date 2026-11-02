# Tasks

## 1. 幅の計算と保存

- [x] 1.1 `web/ui/src/lib/paneWidth.ts`に、幅を範囲に収める関数、保存した値の読み込み、localStorageへの保存、ウィンドウの幅の変化への追従を置く。`paneWidth.test.ts`の単体テストが通ることで確かめた

## 2. 境目

- [x] 2.1 `web/ui/src/pages/InboxLayout.tsx`に、`role="separator"`の境目を置き、ドラッグ、矢印キー、Home、End、ダブルクリックで幅を変える。`paneResize.test.tsx`が通ることで確かめた
- [x] 2.2 `DESIGN.md`のLayoutに、境目の振る舞いと見た目を書き足す

## 3. 通しの確認

- [x] 3.1 web/uiのvitest、typecheck、lint、build、webのGoのvetとテストが通ることを確かめた。PR #65のCIの`verify`も通った
- [x] 3.2 Playwrightとheadless Chromiumで、幅1024px、1280px、1920pxと360pxの画面を開き、横へのスクロールとはみ出しが無いことを測った。結果はPR #65の本文にある
