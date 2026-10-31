# Spec Delta

## MODIFIED Requirements

### Requirement: 着手
始めるラベルが付いたopenのissueがあり、信頼判定を通ったとき、エージェントは、状態を `agent:running` に変えてから実行を始めなければならない（MUST）。
始めるラベルは、`agent:ready`、`agent:plan`、`agent:go` の3つとする。着手のとき、エージェントは、始めるラベルを外して `agent:running` を付ける。

#### Scenario: 着手する
- **WHEN** マスターが作ったissueに、マスターが `agent:ready` を付ける
- **THEN** 次のポーリングでissueは `agent:running` になり、`claude -p` が起動する

#### Scenario: agent:planで着手する
- **WHEN** マスターが作ったissueに、マスターが `agent:plan` だけを付ける
- **THEN** 次のポーリングで `agent:plan` が外れてissueは `agent:running` になり、計画の実行が始まる

### Requirement: プロンプトの内容
エージェントは、issueのタイトルと本文、信頼済みのコメント、エージェント自身の過去の投稿から、プロンプトを作らなければならない（MUST）。
プロンプトのうち、エージェントが追記する指示（終了の宣言の方法、質問の書き方、禁止する操作）は、`--append-system-prompt-file` で渡さなければならない（MUST）。
追記する内容は、実行の種類（計画の実行、承認のあとの実装の実行、それ以外の実装の実行）に応じて変えなければならない（MUST）。

#### Scenario: プロンプトの構成
- **WHEN** 本文とコメント3件（信頼済み2件、非信頼1件）を持つissueで実行を始める
- **THEN** プロンプトには本文と信頼済みのコメント2件だけが入る

#### Scenario: 計画の実行の指示
- **WHEN** モードが `plan` の周期で、承認された計画がまだ無いissueの実行を始める
- **THEN** `--append-system-prompt-file` のファイルには、計画の実行の指示が書かれ、実装の実行の終わり方は書かれていない
