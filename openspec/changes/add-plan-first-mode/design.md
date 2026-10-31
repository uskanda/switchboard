# Design

## Context

動機と範囲はproposal.mdに、判断はgrilling.mdにある。設計の前提になる今の形は、次のとおりである。

- `internal/agent` の `decide` は、状態ラベルが `Ready` なら、`LabelActor` でラベルを付けた人を確かめる。`Waiting` なら、`hasReply` で信頼済みの返答があるかを見る。`hasReply` は、`state=running` 以外の最新のエージェントのマーカーの `ack` より後に、信頼済みのコメントがあるかを見る
- `launch` は、着手のコメントを投稿して取り直す（`claim`、ADR-033）。勝ったら `labels.Swapper` で `agent:running` に付け替え、`execute` から `work` を呼ぶ
- `work` は、`prompt.Build` と `prompt.System` でプロンプトを作る。`settings(verifyCommand)` で権限を作り、`Schema` を渡して `claudecode.Run` を呼ぶ。`waiting` なら `outcome.wait`、`done` なら `finish`（検証、出力検査、push、PR）へ進む
- `outcome` の `wait` は、`permission_denials` があれば `reason=permission` にする
- `internal/labels` は、4つの状態ラベルを固定の配列（`meta`、`ordered`）で持つ。`Normalize` は最も優先順位の高い1つを残す
- `marker.Parse` は、知らない項目を読み飛ばす。項目を足しても、古い版のエージェントは読める
- `workspace.HasSecret` が、コメントの投稿前の秘密の検査である。監査ログは `state.Audit` の `audit.jsonl` にある
- WUIは `<inboxapi>` だけで核とつながり、返答のPOSTで `Origin` と `Host` を確かめる。`inbox serve` は設定ファイルを読んで動く

## Goals / Non-Goals

**Goals:**

- 計画の段階かどうかを、issueのコメントとマーカーだけから毎回導く。端末のローカルの状態に持たない。端末をまたいでも、再起動しても、同じ判定になる
- 計画の実行が、ファイルを1つも残さないことを、権限ではなく実行のあとの検査で保証する
- `plan_first` が偽で、`agent:plan` を使わない利用者には、今と同じ動きのままにする

**Non-Goals:**

- `--resume` でセッションを続けること。各ラウンドは今までどおり新しいセッションで、文脈はissueのコメントから作る
- 実装の実行への追加の指示（`<implement.md>`）、指示の端末をまたいだ同期、指示の以前の版
- Web表示の「承認」ボタン
- ハーネスの成果物（OpenSpec、`grilling.md`）をswitchboardが作ること

## Decisions

### 1. 段階の判定は `decide` で行い、`plan` に持たせる

`decide` は、スレッドを読んだあとに「実行の種類」（`planRun`、`approvedRun`、`directRun`）を決めて `plan` に入れる。手順は次のとおりである。

1. コメントを新しい順にたどり、エージェントの投稿のうち、`state=running` で `mode` を持つ最新のマーカーを探す。その位置から後が今の周期になる
2. 見つからなければ `directRun`。見つかって `mode=direct` でも `directRun`
3. `mode=plan` なら、周期の中で最新の `reason=plan` の待ちのコメントを探す。その後に承認のコメントがあれば `approvedRun`、無ければ `planRun`

始めるラベルでの着手では、手順1の前にモードを決める。`agent:plan` なら `plan`、`agent:go` なら `direct`、`agent:ready` ならプロジェクトの `plan_first` に従う。そのモードを `claim` の着手のコメントに `mode=` として書く。返答による再開の着手のコメントには `mode` を書かない。そうすれば、周期の始まりは「始めるラベルでの着手」に限られる。

代わりに考えた案は次の2つである。

- 端末の状態ディレクトリに段階を記録する。端末をまたいだ実行（ADR-033）と、状態の消失に弱い
- 周期の始まりをラベルの履歴から取る。フォージごとにAPIが違い、GitLabではラベルの事象の取得が要る

### 2. 承認の判定は、信頼判定のあとの文字列の比較だけにする

承認のコメントは、`trust.Instruction` に分類されたコメントだけから探す。`trust` が既に、作成者と最後の編集者を確かめているためである。先頭の空でない行の最初の語が `<plan.approve_word>` と同じ文字列なら、承認とする。`/okay` や `/ok,` は承認にしない。承認を見落としても、計画の実行がもう一度回るだけで済む。誤って承認するよりも害は小さい。

質問への `/ok` は、特別に扱わない。普通の信頼済みの返答として次の `planRun` のプロンプトに入る。計画の実行の指示には、「承認の語だけの返答は推奨どおりの答え、語のあとの上書きはその問いの答え」と書く。

### 3. 計画の実行は、別のスキーマ、別の権限、別のsystem promptで `claudecode.Run` を呼ぶ

`claudecode` の層は変えない。`Request` の `Schema`、`Settings`、`SystemPrompt` を、実行の種類に応じて `internal/agent` で選ぶ。

- スキーマ（`<PlanSchema>`）：`status` は `questions` か `plan`。`questions` は、`title`、`body`、`recommendation` を持つ要素を1つ以上並べた配列とする。`plan` は文字列とする。`summary` は任意の文字列とする。スキーマに合わない宣言は、今の `declaration` と同じく `noDeclaration` の誤りとして扱う
- 権限：許すのは `Read`、`Grep`、`Glob`、`Bash(git status*)`、`Bash(git diff*)`、`Bash(git log*)` である
  - 拒否するのは、`Edit`、`Write`、`NotebookEdit`、`WebFetch` と、`git add`、`git commit`、`git push`、`make`、`gh` のBashである
  - `verify_command` は許さない
- system prompt：`<prompt.PlanSystem>` を新しく作る。組み込みの指示のあとに、追加の指示があれば「利用者の追加の指示」の節を足す。承認のあとの実行には `<prompt.ApprovedSystem>` を作る。今の `prompt.System` に、計画と承認のコメントのIDを名指しする段落を足したものとする

### 4. 計画の実行のあとの検査は、`workspace.Space` に置く

実行の前に `git rev-parse HEAD` を記録しておく。実行のあとに、HEADが同じで、`git status --porcelain --untracked-files=all` が空かを確かめる。違っていれば、`git reset --hard <前のHEAD>` と `git clean -fd` で戻し、`outcome` に「変更を捨てた」ことを足す。無視されたファイル（`.gitignore`）は、もとから検査の外にあるので消さない。

`--permission-mode dontAsk` でも、Bashのリダイレクトは書けてしまう（V18で、allowに無い読み取りのコマンドが実行された）。権限は第一の防壁にすぎないので、この検査を正とする。

### 5. コメントの書式と番号はswitchboardが作る

`outcome` に、質問と計画のコメントを作るメソッドを足す。質問は「**Q4** - **題**」「本文」「➡️ 推奨」の順に並べ、最後に承認の語の使い方の1行を足す。言語は `comments.language` に従う。

番号の始まりは、周期の中の最新の `reason=plan-questions` のマーカーの `q` に1を足した数とする。マーカーが無ければ1とする。コメントの本文を解析して数えるより確かである。

拒否された操作は、`<details>` の中に今の拒否の一覧と同じ形で書く。`reason` は変えない。

### 6. 始めるラベルは「readyの仲間」として `labels` に持つ

`config.Labels` に `Plan` と `Go` を足す。`labels` に `isStart(l)` と `starts(n)` を足し、`StateOf` が始めるラベルを `Ready` の順位で返すようにする。`decide` は、実際に付いている始めるラベルの名前で `LabelActor` を呼ぶ。

`Normalize` は、`running` か `waiting` が付いていれば、今までどおりそれを残す。始めるラベルが最上位で2つ以上付いているときは、変更せずに「始めるラベルが複数」を返す。そのとき `decide` は、ラベルを付けた全員の信頼判定を通ったときに限り、`waiting(reason=error)` の書き戻しをする。信頼されない人のラベルでコメントを書かないためである。

`switchboard labels sync` は6つを作る。既存の4つは `EnsureLabel` が飛ばすので、この機能の前に同期したリポジトリには2つだけが足される。

### 7. 追加の指示は `internal/instructions` に置き、CLIと `inbox serve` から使う

新しいパッケージ `<internal/instructions>` に、置き場所の計算、読み込み、保存を置く。保存は、大きさの検査、`workspace.HasSecret`、0600での書き込み、監査ログへの記録からなる。置き場所の起点は、読み込んだ設定ファイルのディレクトリとする。`config.DefaultPath()` か `--config` の値である。

書き込みは、同じディレクトリの一時ファイルに書いてから `rename` する。エージェントが読む途中で、半分だけ書かれたファイルを見せないためである。

- エージェント：`planRun` の `work` で、全体とプロジェクトの指示を読み、`<prompt.PlanSystem>` に渡す
- CLI：`switchboard instructions edit [<プロジェクト>]`。エディタの起動は、`internal/inbox` の返答で使う `$EDITOR` の処理を共通にして使う
- `inbox serve`：`GET /v1/instructions` と `PUT /v1/instructions` を足す。本文は `{"target": "", "content": "..."}` で、`target` が空なら全体とする。誤りは `too_large`、`secret`、`unknown_project` の種類で返す。`<inboxapi>` に型と誤りを足す
- WUI：`/settings` の画面と、`POST /api/instructions`。`Origin` と `Host` の検査は、返答のPOSTと同じ関数を使う。保存の欄はプロジェクトごとに独立させ、1つの欄の誤りがほかの欄の入力を消さないようにする

### 8. ADRと文書

DECISIONS.mdに、計画を先に立てるモードのADRを1つ足す。内容は、段階をコメントから導くこと、承認をswitchboardが判定すること、計画の実行の検査、追加の指示をローカルのファイルに置くことである。番号は、実装のときのmainの次の番号とする。ADR-019（待ちの定義）は変えない。計画の待ちも、人間への質問を残して終わった状態だからである。

READMEに、始めるラベル、`plan_first`、`/ok`、追加の指示の置き場所と、端末ごとに別のファイルであることを書く。

## Risks / Trade-offs

- ［計画の実行が、許していない読み取りのコマンド（`go test` など）を何度も試し、ターンを使い切る］→計画の実行の指示に、使えるツールを書く。拒否の一覧はコメントの `<details>` に出るので、実物で多ければ許可を足す
- ［承認のつもりの返答の先頭が別の語で、承認にならない（例：「了解、/okです」）］→承認でなければ計画の出し直しになるだけで、実装は始まらない。計画のコメントの最後の1行で書き方を示す
- ［Web表示の設定の画面は、Web表示と同じ端末のファイルしか変えない。エージェントが別の端末にいると、変えたつもりの指示が届かない］→画面に端末の名前を出し、READMEに書く
- ［LANの中の誰でも、追加の指示を書き換えられる］→返答と同じ扱いで、ADR-036で受け入れた範囲に入る。保存は監査ログに残る。指示は計画の実行にだけ渡り、計画の実行はファイルを書けない
- ［`add-run-progress-comment` が先に入り、`agent-run`、`web`、`inbox-api` の同じ所を変える］→実装はそれがmainに入ってから始める（grilling.md）。仕様の差分は別の要件を触るので、衝突は小さい

## Migration Plan

- 設定の項目は、どれも無ければ今と同じ動きになる。既存の設定は書き換えなくてよい
- 計画のモードを使うプロジェクトでは、`switchboard labels sync` を一度実行して、`agent:plan` と `agent:go` を作る
- 古い版のエージェントが新しいマーカーを読んでも、`mode` と `q` を読み飛ばすので、今までどおり動く。ただし、古い版は始めるラベルの `agent:plan` に着手しない。複数の端末では、全部の端末を新しい版にしてから使い始める
- 戻すときは、`agent:plan` を使わず `plan_first` を外せば、今と同じ動きに戻る
