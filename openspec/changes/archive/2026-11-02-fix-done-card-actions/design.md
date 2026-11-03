# Design

## Context

一覧のカードの形では、`<li>`の中が`<Link><Card/></Link>`と`ChangeLink`の兄弟になっていた。カード全体が1件へのリンクなので、PR/MRへのリンクとマージのボタンは枠の外に置くしかなかった。表の形では、`ChangeLink`はもともと同じセルの中にある。

## Goals / Non-Goals

**Goals:**

- リンクの中にリンクやボタンを入れずに、枠だけを共有する

**Non-Goals:**

- 表の形（幅640〜1023px）の見た目は変えない
- 1件の表示のPR/MRの行とマージのボタンは変えない

## Decisions

- 枠（`Card`）を`<li>`の直下に移し、その中に`Link`と`ChangeLink`を兄弟として並べる。`Card`の余白（`p-lg`）は外して`Link`に移すので、線より上は余白も含めて全部が1件へのリンクになる（grilling.mdのQ1）。重ね合わせのリンク（stretched link）は、`z-index`の調整が要り、ボタンの間の隙間を押すと1件の画面へ移るので採らない
- `ChangeLink`に`className`を受け取らせ、カードで使うときだけ`border-t border-outline px-lg pt-sm pb-lg`を付ける。線は完了でPR/MRの行があるカードにだけ出る（Q2）
- 選択中の`border-2 border-primary`は`Card`に付ける。`aria-current`は`Link`に残す。`Link`のフォーカスの枠は`-outline-offset-2`で内側に描き、角を`rounded-md`にして枠からはみ出さないようにする

## Risks / Trade-offs

- 線の下の余白を押しても何も起きない。→ PR/MRのリンクとボタンは高さ44pxなので押しにくくはならない。どこまでが1件へのリンクかは、線で分かる
