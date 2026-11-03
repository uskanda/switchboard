# Design

## Context

1件の見出し（`data-pane-head`）は`sticky`で上に残る。高さは`useHeightVar`が測って`--pane-head-h`へ入れ、カードへのスクロールの`scroll-mt`はこの値で決まる。マージのダイアログは`MergeButton`の中のネイティブの`<dialog>`で、`showModal`で開く。

## Goals / Non-Goals

**Goals:**

- マージと自動マージの取り消しを、どこまでスクロールしても押せる形にする

**Non-Goals:**

- 一覧の行のマージのボタンは変えない
- 出す条件、ダイアログ、トーストは変えない

## Decisions

### D1. 本文の先頭の行を、そのまま見出しの状態の行の下へ移す

リンク、`MergeButton`、`CancelAutoMergeButton`を囲む`flex flex-wrap items-center gap-sm`の行を、見出しの状態の行の直後へ移す。条件は既にある`mergeable`の変数でまとめる。スレッドの末尾にもう1つ置く案は採らない。同じボタンが2か所にでき、自動マージ待ちの表示と`?action=merge`のダイアログを2つ揃えて動かす必要が出るからである。

### D2. ダイアログは見出しの中に置いたままにする

`showModal`で開いた`<dialog>`は最上層（top layer）に出るため、`sticky`と`z-index`の見出しの中にあっても隠れない。ダイアログだけを見出しの外へ出す作りにはしない。

### D3. スクロールの位置の計算には手を入れない

見出しが1行ぶん高くなっても、`--pane-head-h`を測り直すので、カードの`scroll-mt`は追従する。

## Risks / Trade-offs

- 見出しが高くなり、本文の見える範囲が狭くなる → 行は完了で番号とコミットが分かるときだけ出す。狭い画面では`flex-wrap`で折り返し、横にはみ出させない
