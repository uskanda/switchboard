# Tasks

## 1. issue-targetsの`plan_first`

- [x] 1.1 `plan_first`を確かめるテストを、`internal/inboxserve/issues_test.go`と`inboxapi/client_test.go`に足す。落ちることを確かめる
- [x] 1.2 `inboxapi.IssueTarget`に`PlanFirst`を足し、`issueTargets`で設定の値を入れる。`go test ./inboxapi ./internal/inboxserve`と、`web`の`go test ./...`が通ることを確かめる

## 2. 時間の文と進捗の文

- [x] 2.1 `lib/api.test.ts`に`timeText`、`progressText`、`oldestWaiting`のテストを書く。時間の文では、状態ごとの形、1分未満、時刻なしを確かめる。進捗の文では、区切りの種類、区切りの無い行、空の進捗を確かめる。テストが落ちることを確かめる
- [x] 2.2 `lib/api.ts`に`timeText`と`oldestWaiting`を足し、`progressText`を書き換える。2.1のテストが通ることを確かめる
- [x] 2.3 `timeText`を、一覧のカードと1件の見出しの状態の行で使う。既存のテスト（`pages`、`queued`、`usability`）の文言を直し、通ることを確かめる

## 3. 画面の部品

- [x] 3.1 `pages/smallUi.test.tsx`に、この変更の画面の振る舞いのテストを書き、落ちることを確かめる。対象は、ヘッダの件数、カードのタイトル、右のペイン、設定の見出し、プロジェクトの1行、未保存の印、新規issueの補足である
- [x] 3.2 `Header`の待ちの件数を、待ちが1件以上のとき`attention`の太字にする。`live.test.tsx`の件数の確かめ方を読み上げ用の文に直し、通ることを確かめる
- [x] 3.3 `InboxList`のカードのタイトルを2行で切り、全文をtitle属性に持たせる。3.1のテストが通ることを確かめる
- [x] 3.4 `InboxLayout`の右のペインに、最も古い待ちのカードを出す。待ちが無いときは今の文を出す。3.1と`live.test.tsx`のテストが通ることを確かめる
- [x] 3.5 `SettingsPage`に見出し「設定」を置き、節の見出しを`body-md`の太字にする。問題の無いプロジェクトを1行にまとめ、3.1と`projects.test.tsx`のテストが通ることを確かめる
- [x] 3.6 `lib/instructionDrafts.ts`を作り、指示の欄に未保存の印、下書きの復元、`beforeunload`を足す。テストの後始末で下書きを消し、3.1と`plan.test.tsx`が通ることを確かめる
- [x] 3.7 `NewIssue`の`ready`の選択肢に、選んだプロジェクトの`plan_first`の補足を出す。3.1と`newIssue.test.tsx`のテストが通ることを確かめる

## 4. 文書

- [x] 4.1 DESIGN.mdの、ヘッダ、一覧の行、1件の見出し、設定の画面、プロジェクトの節、新規issue、指示の節を直す。色、タイプ、レイアウト、Doの決まりも直し、`designmd lint DESIGN.md`の誤りが0件であることを確かめる

## 5. 全体の確認

- [x] 5.1 `make verify`を通す
- [x] 5.2 `openspec validate web-ui-small-improvements --strict`を通す
