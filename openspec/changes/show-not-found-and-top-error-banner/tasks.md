# Tasks

## 1. ページがありませんの画面

- [ ] 1.1 `notFound.test.tsx`に、`/foo/bar`を開くとヘッダ、「ページがありません」、一覧へのリンク、タブのタイトルが出ることを確かめるテストを先に書く
- [ ] 1.2 `web/ui/src/pages/NotFoundPage.tsx`を作り、`App.tsx`にどのルートにも当たらないときのルートを足す

## 2. 誤りのバナーを画面の上へ移す

- [ ] 2.1 `inboxErrors.test.tsx`に、受信箱のサービスに届かない誤りとフォージの誤りが一覧のペインの外に1つだけ出ることと、設定の画面には出ないことを確かめるテストを先に書く
- [ ] 2.2 `web/ui/src/components/InboxErrors.tsx`を作り、`InboxLayout.tsx`のヘッダの下に置き、`InboxList.tsx`から誤りの表示を消す

## 3. 文書

- [ ] 3.1 `DESIGN.md`の誤りのバナーの項に出す位置と画面を足し、Componentsの項にページがありませんの画面を足す

## 4. 通しの確認

- [ ] 4.1 `make verify`が通ることを確かめる
- [ ] 4.2 `switchboard-web`を幅1280pxと360pxで開き、誤りのあるときとページがありませんの画面を目で確かめる
