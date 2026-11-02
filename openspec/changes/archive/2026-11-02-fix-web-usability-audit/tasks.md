# Tasks

## 1. 1件の画面

- [x] 1.1 `web/ui/src/lib/drafts.ts`を作り、`IssuePane`で書きかけの返答と選んだ答えをissueごとにメモリへ残す。`usability.test.tsx`の「別のissueへ移って戻る」の検査で確かめた
- [x] 1.2 `IssuePane`の返答の欄にCtrl+Enter（⌘+Enter）の送信を足し、空の欄や送信中には送らない。`usability.test.tsx`のCtrl+Enterの検査で確かめた
- [x] 1.3 `IssuePane`で、送れたら「送りました」の行へフォーカスを移し、欄の上の案内を`aria-describedby`で欄に結ぶ。`usability.test.tsx`のフォーカスと説明の検査で確かめた
- [x] 1.4 `Thread`で、エージェントの投稿のアカウント名を出さない。`usability.test.tsx`のスレッドの検査で確かめた
- [x] 1.5 `PlanQuestions`で、推奨に当たる選択肢の後ろへ「（推奨）」を付ける。`usability.test.tsx`の計画の質問の検査で確かめた

## 2. 一覧

- [x] 2.1 `lib/api.ts`に`progressText`と`plainText`を置き、実行中の行の進捗と質問の冒頭の表示を直す。`usability.test.tsx`の一覧の検査で確かめた
- [x] 2.2 `InboxList`で、待ちの節に見出し「返答待ち」を付け、完了のPR/MRのリンクを番号の名前で出す。`usability.test.tsx`の一覧の検査で確かめた

## 3. 画面全体

- [x] 3.1 `components/Loading.tsx`を作り、一覧、設定の各節、新規issueのダイアログで読み込み中に出す。`usability.test.tsx`の読み込みの検査と、直した`issueLoading.test.tsx`で確かめた
- [x] 3.2 `lib/title.ts`を作り、一覧、1件、設定の画面でタブのタイトルを付ける。`usability.test.tsx`のタブのタイトルの検査で確かめた
- [x] 3.3 `Header`の件数を`sr-only`の文にし、幅640px未満で設定のリンクを1行目に残す。`done.test.tsx`と`queued.test.tsx`の件数の検査を読み上げ用の文に合わせて直した
- [x] 3.4 `index.css`でルートの文字の大きさを16pxにし、入力欄と選択欄の枠をトークン`field-outline`にする。`DESIGN.md`を改訂し、`tokens.css`を`pnpm tokens`で作り直した

## 4. 通しの確認

- [x] 4.1 `make verify`が通ることを確かめた。PR #93のCIの`verify`も通った
- [x] 4.2 `switchboard-web`のビルドした画面をモックのデータで表示し、幅390pxと1440pxの見た目と、ボタンの高さが44pxであることを確かめた
- [ ] 4.3 `switchboard-web`を実データで開き、実機のスマホでも上の変更を確かめる
