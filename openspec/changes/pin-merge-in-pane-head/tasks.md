# Tasks

## 1. テスト（先に失敗させる）

- [x] 1.1 `web/ui/src/pages/merge.test.tsx`を書き換える。1件の表示のマージのボタンとPRのリンクが`[data-pane-head]`の中にあり、見出しの外に無いことを確かめる。失敗することも確かめる
- [x] 1.2 `merge.test.tsx`の自動マージ待ちのテストで、「自動マージを取り消す」が`[data-pane-head]`の中にあることを確かめる。失敗することも確かめる

## 2. 実装

- [x] 2.1 `IssuePane.tsx`で、PR/MRのリンク、`MergeButton`、`CancelAutoMergeButton`の行を、見出しの状態の行の下へ移す。条件は`mergeable`でまとめる（design D1）
- [x] 2.2 `IssuePane.tsx`の先頭のコメントに、見出しに残るものとしてPR/MRのリンクとマージのボタンを加える

## 3. 文書

- [x] 3.1 `DESIGN.md`の「1件の見出し」「マージ」「自動マージ」の置き場所を、固定の見出しの状態の行の下に書き換える

## 4. 通しの確認

- [x] 4.1 `make verify`を通す
