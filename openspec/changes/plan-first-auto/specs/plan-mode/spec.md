## MODIFIED Requirements

### Requirement: モードの決定
始めるラベルで着手するとき、エージェントは、実行のモードを次のとおり決めなければならない（MUST）。
`agent:plan` なら計画のモード（`plan`）、`agent:go` なら直ちに実装するモード（`direct`）とする。`agent:ready` なら、プロジェクトの `<projects[].plan_first>` が `always` のとき `plan`、`auto` のとき必要なら計画するモード（`auto`）、`never` のとき `direct` とする。
エージェントは、決めたモードを、その着手のコメントのマーカーの `mode` に記録しなければならない（MUST）。

#### Scenario: plan_firstのプロジェクトでagent:readyを付ける
- **WHEN** `<projects[].plan_first>` が `always` のプロジェクトで、マスターがissueに `agent:ready` を付ける
- **THEN** 着手のコメントのマーカーに `mode=plan` が入り、計画の実行が始まる

#### Scenario: plan_firstがautoのプロジェクトでagent:readyを付ける
- **WHEN** `<projects[].plan_first>` が `auto` のプロジェクトで、マスターがissueに `agent:ready` を付ける
- **THEN** 着手のコメントのマーカーに `mode=auto` が入り、計画の実行が始まる

#### Scenario: plan_firstでないプロジェクトでagent:planを付ける
- **WHEN** `<projects[].plan_first>` が `never` のプロジェクトで、マスターがissueに `agent:plan` だけを付ける
- **THEN** 着手のコメントのマーカーに `mode=plan` が入り、計画の実行が始まる

#### Scenario: agent:goを付ける
- **WHEN** `<projects[].plan_first>` が `auto` のプロジェクトで、マスターがissueに `agent:go` を付ける
- **THEN** 着手のコメントのマーカーに `mode=direct` が入り、今までと同じ実装の実行が始まる

### Requirement: 計画の段階の判定
モードが `plan` か `auto` の周期の間、今の周期に承認された計画が無いとき、エージェントは、次の実行を計画の実行にしなければならない（MUST）。
承認された計画とは、今の周期の最新の計画のコメント（`reason=plan` の待ちのコメント）より後に、承認のコメントがあるものをいう。
今の周期に承認された計画があるとき、エージェントは、次の実行を実装の実行にしなければならない（MUST）。

#### Scenario: 質問への返答
- **WHEN** 今の周期に計画のコメントがまだ無いissueで、マスターが質問に返答する
- **THEN** 次の実行は計画の実行になる

#### Scenario: 前の周期で承認した計画
- **WHEN** 前の周期で計画が承認されて完了したissueに、マスターが再び `agent:plan` を付ける
- **THEN** 前の周期の承認は数えられず、計画の実行が始まる

#### Scenario: autoの周期で計画を承認する
- **WHEN** `mode=auto` で始めた周期で、計画のコメントのあとに、マスターが `/ok` とコメントする
- **THEN** 次の実行は承認された計画の実装の実行になる

### Requirement: 計画の実行の終了の宣言
計画の実行では、エージェントは、実装の実行とは別のスキーマを `--json-schema` で渡さなければならない（MUST）。
宣言は `status`（`questions` か `plan`、計画の段階を省いてよい実行ではそれに加えて `implement`）を必ず持つ。`questions` のときは、題、本文、推奨の答えを持つ質問を1つ以上並べた配列を持つ。`plan` と `implement` のときは、計画の本文を持つ。
計画の段階を省いてよい実行でないとき、エージェントは、スキーマの `status` に `implement` を入れてはならない（MUST NOT）。
宣言が無いか、スキーマに合わないとき、エージェントは、実行を `waiting(reason=error)` として扱わなければならない（MUST）。

#### Scenario: 質問を宣言する
- **WHEN** 計画の実行が、`status: questions` と質問2つを宣言して終わる
- **THEN** resultの `structured_output` から、2つの質問の題、本文、推奨の答えが読み取られる

#### Scenario: doneを宣言する
- **WHEN** 計画の実行が `status: done` を宣言して終わる
- **THEN** スキーマに合わないので、issueは `waiting(reason=error)` になる

#### Scenario: 省いてよくない実行がimplementを宣言する
- **WHEN** `agent:plan` で始めた計画の実行が `status: implement` を宣言して終わる
- **THEN** 渡したスキーマに `implement` は無く、issueは `waiting(reason=error)` になる

#### Scenario: 計画の本文の無いimplement
- **WHEN** 計画の段階を省いてよい実行が、計画の本文を持たない `status: implement` を宣言して終わる
- **THEN** issueは `waiting(reason=error)` になり、実装の実行は始まらない

## ADDED Requirements

### Requirement: 計画の段階を省いてよい実行
モードが `auto` の周期で、今の周期に質問のコメント（`reason=plan-questions`）も計画のコメント（`reason=plan`）も無いとき、エージェントは、その計画の実行を、計画の段階を省いてよい実行にしなければならない（MUST）。
それ以外の計画の実行は、計画の段階を省いてよい実行にしてはならない（MUST NOT）。

#### Scenario: autoの周期の最初の計画の実行
- **WHEN** `<projects[].plan_first>` が `auto` のプロジェクトで、マスターがissueに `agent:ready` を付ける
- **THEN** 最初の計画の実行に渡すスキーマの `status` に `implement` がある

#### Scenario: 質問のあとの計画の実行
- **WHEN** `mode=auto` で始めた周期で、エージェントが質問し、マスターが `/ok` と返答する
- **THEN** 次の計画の実行に渡すスキーマの `status` に `implement` は無い

#### Scenario: 誤りで終わったあとの返答
- **WHEN** `mode=auto` で始めた周期の最初の計画の実行が `waiting(reason=error)` で終わり、マスターが返答する
- **THEN** 次の計画の実行に渡すスキーマの `status` に `implement` がある

### Requirement: 計画の段階を省く指示
計画の段階を省いてよい実行のsystem promptに、エージェントは次の指示を書かなければならない（MUST）。
省くのは、issueと信頼済みのコメントで設計上の判断がすべて決まっていて、聞く質問が1つも無いときだけであること。省くときは `status: implement` で、計画の本文と題を宣言すること。迷ったら `questions` で質問すること。

#### Scenario: 省いてよい実行を始める
- **WHEN** 計画の段階を省いてよい実行を始める
- **THEN** `--append-system-prompt-file` のファイルに、省く条件、`implement` の書き方、迷ったら質問することが書かれている

#### Scenario: 省いてよくない実行を始める
- **WHEN** `agent:plan` で始めた計画の実行を始める
- **THEN** `--append-system-prompt-file` のファイルに、`implement` の書き方は書かれていない

### Requirement: 計画の段階を省いたコメント
計画の実行が `implement` を宣言したとき、エージェントは、計画の題、要件が明確なので質問と承認を省いて実装すること、計画の本文を書いた1件のコメントを投稿しなければならない（MUST）。
このコメントのマーカーは、`state=running`、`mode=direct`、`reason=plan-skipped` と、続く実装の実行の `run` を持たなければならない（MUST）。
エージェントは、このとき、issueを `agent:waiting` にしてはならず、待ちのコメントも投稿してはならない（MUST NOT）。

#### Scenario: 要件が明確なissue
- **WHEN** `mode=auto` の最初の計画の実行が、`status: implement` と計画を宣言して終わる
- **THEN** 計画の本文と、質問と承認を省いて実装することを書いたコメントが、`mode=direct reason=plan-skipped` のマーカーで投稿され、issueは `agent:running` のままである

### Requirement: 計画の段階を省いたあとの実装の実行
計画の段階を省いたコメントを投稿したあと、エージェントは、順番待ちに戻さず、同じスロットのまま、実装の実行を始めなければならない（MUST）。
この実装の実行のsystem promptで、エージェントは、省いたコメントを計画として名指ししなければならない（MUST）。system promptには、その計画を実装すること、決まったことを蒸し返さないこと、計画に無い判断が新しく出たときだけ質問することを書く。
この実装の実行には、エージェントは、直ちに実装する実行と同じ権限、ツール、追加の指示、画像の取り決めを渡さなければならない（MUST）。

#### Scenario: 省いて実装に入る
- **WHEN** `mode=auto` の最初の計画の実行が `implement` を宣言する
- **THEN** 続けて実装の実行が始まり、そのsystem promptに省いたコメントが名指しされ、doneの投稿の `run` は省いたコメントの `run` と同じである

### Requirement: 計画の段階を省いたあとの待ち
今の周期で計画の段階を省いたとき、エージェントは、それ以後の待ちを、モードが `direct` の待ちと同じに扱わなければならない（MUST）。

#### Scenario: 省いたあとの質問に返答する
- **WHEN** 計画の段階を省いたあとの実装の実行が、質問を残して終わり、マスターが返答する
- **THEN** 次の実行は直ちに実装する実行になる
