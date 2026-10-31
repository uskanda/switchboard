# Proposal

## Why

`gh:uskanda/switchboard` #19では、手作業なら `/spec` から始めるはずの変更を、エージェントが質問を1つもせずに実装し、PRまで出した（#22）。switchboardのプロンプトは「issueの作業をして、一度で終える」形で、聞くのは要件が曖昧なときの質問1つだけである。そのため、設計を詰めてから実装する進め方を、無人の実行で再現できない。手作業と同じ進め方を、ハーネスに頼らずswitchboardの側で持たせる。

## What Changes

- 計画を先に立てるモードを足す。計画の段階では、エージェントはissueのコメントだけで質問のラウンドを往復し、質問が尽きたら計画をコメントして、承認まで実装しない
  - 計画の実行は、読み取りだけの権限で動く。実行のあとにHEADとworktreeを確かめ、変更があれば捨てる
  - 計画の実行には別の終了の宣言（`questions` か `plan`）を渡す。質問には推奨の答えを付け、番号はラウンドをまたいで通し番号にする。コメントの書式はswitchboardが整える
  - 信頼済みの人のコメントの先頭行が承認の語（既定は `/ok`）なら、計画への返答では承認として実装に入る。質問への返答では、推奨どおりの答えとして次のラウンドへ進む
  - 承認のあとの実装の実行には、承認された計画と承認のコメントを名指しで伝える
- 始めるラベルを3つにする。付けるラベルは常に1つで済む
  - `agent:ready` はプロジェクトの `<projects[].plan_first>` に従う（既定は `false` で、今と同じ動き）
  - `agent:plan` は計画から始め、`agent:go` はすぐに実装する
  - 着手したら `agent:running` に付け替え、モードは着手のコメントのマーカーに残す
- 待ちの理由に `plan-questions`（計画の質問）と `plan`（計画の承認待ち）を足す。状態ラベルは今の4つのまま。計画の実行で権限が拒否されても、理由は変えない
- 計画の実行に渡す利用者の追加の指示を足す
  - 全体に1つと、プロジェクトごとに1つ。設定ファイルと同じディレクトリの `instructions/` の下に置く
  - `switchboard instructions edit [<プロジェクト>]` で、`$EDITOR` を使って編集できる
  - `switchboard inbox serve` のAPIと、Web表示の設定の画面からも編集できる。保存は、大きさの上限と秘密の検査を通し、監査ログに残す
- DECISIONS.mdに、計画を先に立てるモードのADRを足す。READMEに使い方を書く

## Capabilities

### New Capabilities

- `plan-mode`: 計画の段階の判定、計画の実行の権限と実行後の検査、計画の実行の終了の宣言とコメントの書式、承認の判定、承認のあとの実装の実行
- `plan-instructions`: 追加の指示のファイルの置き場所と読み込み、CLIでの編集、保存の検査と記録

### Modified Capabilities

- `agent-run`: 始めるラベル3つでの着手、着手のコメントのマーカーのモード、段階に応じたプロンプト
- `state-labels`: 始めるラベルの作成と、状態ラベルの正規化での扱い
- `trust`: 始めるラベルを付けた人の信頼判定
- `run-outcome`: 待ちの理由の追加、計画の実行での権限の拒否の扱い、マーカーの項目の追加
- `config`: `<projects[].plan_first>`、`<labels.plan>`、`<labels.go>`、`<plan.approve_word>`
- `notify`: 計画の待ちも通知する
- `inbox-api`: 追加の指示の読み出しと保存
- `web`: 計画の待ちの理由の表示と、追加の指示を編集する設定の画面

## Impact

- `internal/agent`：着手、段階の判定、計画の実行の起動と実行後の検査、書き戻し
- `internal/prompt`：計画の実行と承認のあとの実行のsystem prompt、追加の指示の節
- `internal/claudecode`：計画の実行用のスキーマと権限
- `internal/marker`：`mode` などの項目。未知の項目は今も読み飛ばすので、古いエージェントとも共存できる
- `internal/labels`、`internal/config`、`internal/trust`、`internal/notify`
- `internal/inboxserve`、`inboxapi`：追加の指示のAPI
- `cmd/switchboard`：`switchboard instructions edit`
- `web/`：設定の画面と、理由の表示
- 先に `add-run-progress-comment`（#23）をmainに入れ、この変更の実装はそのあとに始める。#22は取り込まないので、考えに入れない
