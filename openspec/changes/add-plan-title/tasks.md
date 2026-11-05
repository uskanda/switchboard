# Tasks

## 1. 計画の宣言とコメントの書式

- [x] 1.1 `internal/agent/planrun_test.go`に、質問と計画のコメントの見出しの次の段落が`**計画：** 〈題〉`（英語は`**Plan:** 〈title〉`）になり、`postfmt.Question`がその段落を返すテストを足す。題の無い宣言と空白だけの題が`waiting(reason=error)`になるテスト、題の無い古いコメントで`postfmt.Question`が今までの段落を返すテストも足す。既存のテストの宣言に`title`を足す
- [x] 1.2 `PlanSchema`と`PlanDeclaration`に`title`を足し、`planDeclaration`で空白だけの題を異常とする。`messages.go`に`planLabel`を足し、`planQuestions`と`planText`の先頭に題の段落を置く。1.1のテストが通ることを`make verify`で確かめる

## 2. 通知

- [x] 2.1 `internal/agent/notify_test.go`の計画の待ちの通知のテストで、質問の待ちと計画の待ちの両方に題が載ることを確かめるよう直す
- [x] 2.2 `writeback.go`の通知で、`plan-questions`の本文を最初の`❓`から切る処理を外す。2.1のテストが通ることを`make verify`で確かめる

## 3. 受信箱の抜粋

- [x] 3.1 `internal/inbox/excerpt_test.go`に、`plan`と`plan-questions`の投稿で`Excerpt`が`**計画：** 〈題〉`を返し、長い題が72字で切られ、題の無い古い投稿で今までの段落を返すテストを足す。`make verify`で通ることを確かめる

## 4. 計画の実行への指示

- [x] 4.1 `internal/prompt/prompt_test.go`に、`PlanSystem`が`"title"`を必須の欄として書き、Languageの行に`"title"`を含むことを確かめるテストを足す
- [x] 4.2 `PlanSystem`の「How to end the run」に`title`の行を足し、Languageの行を直す。4.1のテストが通ることを`make verify`で確かめる

## 5. Web UI

- [x] 5.1 `web/ui/src/pages/usability.test.tsx`に、質問の冒頭が`**計画：** 設定の読み込みを分ける`の計画の待ちのカードで「計画： 設定の読み込みを分ける」が出るテストを足す。`make verify`で通ることを確かめる

## 6. 全体の確認

- [x] 6.1 `make verify`を通し、`openspec validate add-plan-title --strict`が通ることを確かめる
