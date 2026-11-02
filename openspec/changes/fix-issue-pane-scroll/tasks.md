# Tasks

## 1. 文書

- [x] 1.1 `DESIGN.md`のLayoutとComponentsに、固定の見出しと自動更新の知らせを足す

## 2. テスト（先に失敗させる）

- [x] 2.1 `web/ui/src/test-observers.ts`に、`scrollIntoView`とIntersectionObserverのフェイクを置き、`test-setup.ts`で入れる
- [x] 2.2 `web/ui/src/lib/scrollTarget.test.ts`に、`targetOf`と`addedCards`の単体テストを書く
- [x] 2.3 `web/ui/src/pages/scroll.test.tsx`に、開いた直後のスクロール、キャッシュからの開き方、知らせの出し方と消え方、固定の見出しのテストを書き、失敗することを確かめる

## 3. 実装

- [x] 3.1 `lib/scrollTarget.ts`を作り、`Thread`の各カードに`data-index`を付ける（design D1）
- [x] 3.2 `IssuePane`で、開いた直後のスクロールと知らせを実装する（design D2）
- [x] 3.3 `Header`と`IssuePane`で高さを測り、見出しを固定し、右のペインの上の余白を見出しへ移す（design D3）

## 4. 通しの確認

- [x] 4.1 `make verify`が通ることを確かめる
- [ ] 4.2 `switchboard-web`をブラウザの幅1280pxと360pxで開いて確かめる。確かめるのは、質問のカードの位置（見出しの直下）、見出しの固定とタイトルの切り替え、横へのスクロールの有無、知らせの表示と行き先である
- [ ] 4.3 `specs/web/spec.md`の差分を、実装が済んだらmain specsへ反映してarchiveする
