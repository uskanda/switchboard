# Tasks

## 1. ヘッダの名前

- [x] 1.1 `web/ui/src/components/Header.tsx`の名前を「SwitchBoard Inbox」にし、幅640px未満では後ろのInboxの部分を隠す。`pages.test.tsx`の見出しの検査を新しい名前に直し、vitestが通ることで確かめた
- [x] 1.2 `DESIGN.md`のComponentsのヘッダの記述を新しい名前に直す。mainとのコンフリクトは、自動更新の文言の記述と1つにまとめて解消した（マージコミット21487c2）

## 2. 通しの確認

- [x] 2.1 `make verify-web`が通ることを確かめた。PR #64のCIの`verify`も通った
- [ ] 2.2 `switchboard-web`をブラウザの幅1280pxと360pxで開き、ヘッダの名前の出方を目で確かめる
