# Tasks

## 1. プロジェクトでまとめる処理

- [x] 1.1 `web/ui/src/lib/group.test.ts`を先に書き、失敗することを確かめる（GitHubとGitLabの同名のプロジェクトを分ける、サブグループの順と中の行の順、空の一覧）
- [x] 1.2 `web/ui/src/lib/group.ts`に`projectRef`と`groupByProject`を書き、1.1のテストが通ることを確かめる

## 2. まとめるかどうかの覚え方

- [x] 2.1 `web/ui/src/lib/groupPref.test.ts`を先に書き、失敗することを確かめる（初めの値はオン、`"false"`のときだけオフ、切り替えると保存される）
- [x] 2.2 `web/ui/src/lib/groupPref.ts`に`readGroupByProject`と`useGroupByProject`を書き、2.1のテストが通ることを確かめる

## 3. 一覧の画面

- [x] 3.1 `web/ui/src/pages/grouping.test.tsx`を先に書き、失敗することを確かめる（見出しの順と件数、サブグループの中の行、実行中と完了の見出し、選んだ行の印、トグル）
- [x] 3.2 `InboxList.tsx`にトグルとサブグループの描き方を足し、`NewIssue.tsx`の囲みを外して、3.1のテストと既存のWUIのテストが通ることを確かめる
- [x] 3.3 `DESIGN.md`の「一覧の節」と「新規issue」の項に、サブグループとトグルを書き足し、足した文にtextlintの指摘が無いことを確かめる

## 4. 全体の確認

- [x] 4.1 `make verify`が通ることを確かめる
- [x] 4.2 1280px、800px、375pxの幅のブラウザで実際の画面を開き、見出しの並び、横スクロールが無いこと、トグルと「新規issue」が1行に収まること、オンとオフで表示が変わることを確かめる
- [x] 4.3 `openspec validate 2026-11-03-group-inbox-by-project --strict`が通ることを確かめる
