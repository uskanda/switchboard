# Tasks

## 1. 待ちを選ぶ関数

- [x] 1.1 `web/ui/src/lib/sent.ts`を作り、送った待ちの印をメモリに持つ。`test-setup.ts`で検査ごとに消す
- [x] 1.2 `web/ui/src/lib/waits.ts`に`nextWait`、`stepWait`、`ignoreKey`を作る。`waits.test.ts`で、回り込みと今のissueの除外を確かめる。送った待ちを飛ばし、新しい待ちは飛ばさないことも確かめる。端で止まることと、開いていないときの動きも確かめる

## 2. 1件の表示

- [x] 2.1 `IssuePane`で、送れたら待ちに印を付け、「次の待ちへ」のボタンか「他に待ちはありません」を出し、フォーカスをボタンか「送りました」の行へ移す。返答の欄の`aria-keyshortcuts`に`r`を足す。`waitNav.test.tsx`と、直した`usability.test.tsx`の検査で確かめる

## 3. 一覧と画面全体

- [x] 3.1 `InboxLayout`で`j`、`k`、`r`のキーを受け、一覧の下にキーの案内を出す。`waitNav.test.tsx`で、移る動き、端、実行中を通らないこと、受けないとき、`r`、案内の1行を確かめる
- [x] 3.2 `DESIGN.md`のLayoutとComponentsに、キーの案内、「次の待ちへ」、`r`を書き足す

## 4. 通しの確認

- [x] 4.1 `make verify`が通ることを確かめる
- [x] 4.2 `openspec validate add-web-wait-navigation --strict`が通ることを確かめる
