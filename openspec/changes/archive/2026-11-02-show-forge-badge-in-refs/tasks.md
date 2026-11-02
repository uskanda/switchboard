# Tasks

## 1. 色と部品

- [x] 1.1 DESIGN.mdにforgeの色（明るいテーマと暗いテーマ）と`badge-github`、`badge-gitlab`を足し、残っていたマージの印を消す。`designmd lint DESIGN.md`で誤りが0件であることを確かめる
- [x] 1.2 `pnpm tokens`で`tokens.css`を書き出し、`index.css`の暗いテーマに`--color-forge-*`の差し替えを足す。`pnpm build`が通ることを確かめる
- [x] 1.3 `lib/ref.ts`に`splitRef`と`refText`を足す。gh、gl、`#`の有無、形の合わない文字列を`ref.test.ts`で確かめる
- [x] 1.4 Badgeにtone `github`と`gitlab`を足し、`components/IssueRef.tsx`に`IssueRef`と`ForgeBadge`を作る。`pnpm typecheck`が通ることを確かめる

## 2. 表示の置き換え

- [x] 2.1 一覧の表とカード、1件の表示の見出しを`IssueRef`にする。`pages.test.tsx`と`scroll.test.tsx`で、`gh:o/r#12`が出ず、「o/r#12」と「GitHub」が並ぶことを確かめる
- [x] 2.2 新規issueの選択肢を「o/r GitHub（担当：desk）」に、作成のトーストを`IssueRef`にする。`newIssue.test.tsx`で確かめる
- [x] 2.3 プロジェクトの節のカードと追加のトーストを`IssueRef`にし、入力欄の形は残す。`projects.test.tsx`で確かめる
- [x] 2.4 指示の欄の名前を`IssueRef`に、保存のトーストの文を`refText`の形にする。`plan.test.tsx`で欄の名前「o/r GitHub」「nas/sb GitLab」とトーストの文を確かめる

## 3. 全体の確認

- [x] 3.1 `make verify-web`が通ることと、リンク先とAPIに送る`ref`が変わっていないことを既存のテストで確かめる
- [x] 3.2 `openspec validate show-forge-badge-in-refs --strict`が通ることを確かめる
