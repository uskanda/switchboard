# Tasks

## 1. テスト

- [x] 1.1 `newIssue.test.tsx`に、閉じた欄のアイコンとバッジ、一覧の各選択肢のアイコンと名前、キーで選ぶこと、Escapeで一覧だけが閉じることのテストを足し、失敗を確かめる
- [x] 1.2 `newIssue.test.tsx`、`smallUi.test.tsx`、`pending.test.tsx`のうち、ネイティブの`<select>`を前提にしたテストを、引き金を押す操作に書き換える

## 2. 実装

- [x] 2.1 `web/ui`に`@radix-ui/react-select`を足し、`pnpm-lock.yaml`を更新する
- [x] 2.2 `components/ui/select.tsx`を足す。一覧をダイアログの中へportalし、開いている間はダイアログの`cancel`を止める
- [x] 2.3 `NewIssue.tsx`の`<select>`を置き換え、引き金と各選択肢に`IssueRef`と担当の端末を出す。各選択肢に「o/r GitHub（担当：desk）」の`aria-label`を付ける

## 3. 文書

- [x] 3.1 DESIGN.mdの参照、プロジェクトのアイコン、新規issueの項を直す
- [x] 3.2 `make verify`を通す
