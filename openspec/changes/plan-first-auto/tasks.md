# Tasks

## 1. 設定の `plan_first` の3値

- [x] 1.1 `internal/config/plan_test.go` に、5つの値の読み取り、知らない値の拒否、キーが無いとき、`Plans()` のテストを足し、`edit_test.go` を `plan_first: auto` の書き戻しに改める
- [x] 1.2 `config.PlanFirst` と `UnmarshalYAML`、`Mode()`、`Plans()` を作り、`NewProject` と `insert` を文字列で書くよう改める。1.1のテストが `make verify` で通ることを確かめる
- [x] 1.3 `config/switchboard.example.yaml` の `plan_first` の説明を3値に改める

## 2. モードの決定と分類

- [x] 2.1 `planmode_test.go` に、ラベルと3値の組み合わせごとのモード、`auto` の最初の実行だけに省略を許すこと、省略のあとの返答が直ちに実装する周期になることのテストを足す
- [x] 2.2 `marker` に `ModeAuto` と `PlanSkipped` を足し、`modeFor`、`classify`、`plan.skippable` を実装する。2.1のテストが通ることを確かめる

## 3. 計画の実行の宣言と省略のあとの実装の実行

- [x] 3.1 `prompt_test.go` に `PlanSkipSystem` と `SkippedSystem` のテストを、`planskip_test.go` に偽のclaudeを使う流れのテスト（省略のコメント、同じ着手での実装の実行、system promptの名指し、`implement` を許さない実行、計画の無い `implement`、省略のあとの返答、日本語のコメント）を足す
- [x] 3.2 `PlanSkipSchema`、`planDeclaration` の `implement`、`PlanSkipSystem`、`SkippedSystem`、`outcome.skip`、`planSkip`、`skipAgain`、`skippedRun` とメッセージを実装する。3.1のテストが通ることを確かめる

## 4. 受信箱のAPIとWeb表示

- [x] 4.1 `inboxapi` の `Project`、`IssueTarget`、`AddProjectRequest` に `plan_first_mode` と `Mode()` を足し、`inboxserve` で返し、受ける。`client_test.go`、`inboxserve` の `issues_test.go` と `projects_test.go`、`web/internal/server/projects_test.go` のテストで確かめる
- [x] 4.2 `projects.test.tsx` と `smallUi.test.tsx` に、3つの選択肢と初期値 `auto`、送る値、3値と古い真偽値の表示のテストを足す
- [x] 4.3 `ProjectsSection.tsx`、`NewIssue.tsx`、`lib/api.ts` を3値に改める。4.2のテストが通ることを確かめる

## 5. 文書と全体の確認

- [x] 5.1 `docs/GUIDE.md` のラベルの説明と設定の例、`docs/DECISIONS.md` のADR-064、`docs/examples/openspec.md` の指示の例を改める
- [x] 5.2 `make verify` と `openspec validate plan-first-auto --strict` が通ることを確かめる
