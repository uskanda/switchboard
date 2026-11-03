# Tasks

## 1. 一覧のカード

- [x] 1.1 `merge.test.tsx`に、左のペインの完了のカードで、`PR #9`のリンクとマージのボタンが枠の中にあり、`aria-current="page"`の1件へのリンクの中に無いことの検査を足す。今のコードで失敗することを確かめた
- [x] 1.2 `InboxList`の`Items`のカードを、`Card`の中に`Link`と`ChangeLink`を兄弟として並べる組み方に変え、`ChangeLink`に`className`を受け取らせて線と余白を付ける。1.1の検査と、既存のmerge、done、usabilityの検査が通ることを`pnpm exec vitest run`で確かめた

## 2. 文書

- [x] 2.1 `DESIGN.md`の一覧の行とマージの項に、完了のカードの組み方を書く。textlintで確かめた

## 3. 通しの確認

- [x] 3.1 `make verify`が通ることを確かめた
- [ ] 3.2 `switchboard-web`を開き、狭い画面と左のペインで、PR/MRの行が枠の中で線に区切られていること、フォーカスの枠と選択中の枠がはみ出さないことを確かめる
