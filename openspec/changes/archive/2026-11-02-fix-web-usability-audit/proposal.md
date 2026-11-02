# Proposal

## Why

前提知識の無いサブエージェントにWUIの使いやすさを監査させたところ、小さな問題が多く見つかった。たとえば、別のissueへ移ると返答の書きかけが消え、一覧の実行中の行には意味の無い `?` が出ていた。重要度が中以上で手間の小さいものを、PR #93でまとめて直した。

## What Changes

1件の画面について、次のとおり直す。

- issueごとの書きかけの返答と選んだ答えを、メモリへ残す。ブラウザのストレージには書かない
- 返答の欄でCtrl+Enter（⌘+Enter）を押すと、「返答を送る」と同じく欄の文を送る
- 送れたら、「送りました」の行へフォーカスを移す
- 返答の欄の上の案内を、欄の説明として読み上げさせる
- エージェントの投稿では、アカウント名を出さずに「エージェント」の印だけを出す
- 計画の質問で、推奨に当たる選択肢へ「（推奨）」を付ける

一覧について、次のとおり直す。

- 実行中の行で、進捗の `?` を「進捗はまだありません」に置き換える
- 質問の冒頭から、Markdownの太字とコードの記号を除く
- 待ちの節に「返答待ち」の見出しを付ける
- 完了のPR/MRのリンクを`PR #N`（GitLabでは `MR !N`）と書く。1件の画面のリンクも同じにする

画面全体について、次のとおり直す。

- 一覧、設定の各節、新規issueのダイアログで、読み込み中に `読み込んでいます` を出す
- タブのタイトルに、画面の名前と待ちの件数を出す
- ヘッダの件数を、読み上げ用の文として出す
- 幅640px未満のヘッダで、設定のリンクを1行目の右端に置く
- ルートの文字の大きさを16pxにし、入力欄と選択欄の枠を新しいトークン`field-outline`にする

監査の残りの項目は、#82〜#92のissueに分けた（grilling.mdの「後回しにしたもの」）。

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- `web`: 返答の書きかけの保持と送り方、エージェントの投稿の書き手、推奨の印、一覧の行と節の表示、読み込み中の表示、タブのタイトル、ヘッダの件数の読み上げ

## Impact

- `web/ui/src/components/` の既存のコンポーネント：`Header.tsx`、`InboxList.tsx`、`IssuePane.tsx`
- 同じく：`NewIssue.tsx`、`PlanQuestions.tsx`、`ProjectsSection.tsx`
- 同じく：`Thread.tsx`、`ui/input.tsx`、`ui/textarea.tsx`
- 新規：`web/ui/src/components/Loading.tsx`、`web/ui/src/lib/drafts.ts`、`web/ui/src/lib/title.ts`
- `web/ui/src/lib/api.ts`、`web/ui/src/pages/InboxLayout.tsx`、`web/ui/src/pages/SettingsPage.tsx`
- `web/ui/src/index.css`、`web/ui/src/tokens.css`
- `web/ui/src/pages/usability.test.tsx`（新規）と、既存の3つのテスト
- `DESIGN.md` のcolorsとComponents
