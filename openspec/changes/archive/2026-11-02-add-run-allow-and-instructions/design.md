# Design

## Context

ファイルを書く実行の権限は、`internal/agent/run.go` の `settings(verifyCommand)` が組み立てている。許可は `Read`、`Edit`、`Write`、`Glob`、`Grep`、gitの一部、検証のコマンドだけである。`--permission-mode dontAsk` で動くので、許可に無いツールは確認なしに拒否される。利用者の追加の指示は、計画の実行の `PlanSystem` にだけ「Additional instructions from the user」の節として入る。保存は `internal/instructions` の `Store` が担い、置き場所は `plan.md` に固定されている。受信箱のAPIの `Instruction` は `target` と `content` だけを持つ。

grilling（`grilling.md`）で、製品はOpenSpecを知らないまま、汎用の2つの仕組みを足すと決めた。追加の許可と、実装用の追加の指示である。

## Goals / Non-Goals

**Goals:**
- 設定に書いた追加の許可を、ファイルを書く実行に足す
- 実装用の指示 `<run.md>` を、計画用と同じ方法で保存、編集し、ファイルを書く実行に渡す
- OpenSpecを使うプロジェクトでの使い方を、記入例として文書に載せる

**Non-Goals:**
- 計画の実行の権限を広げること（後回し）
- 実行ごとに、使った追加の許可と指示のハッシュを監査ログに残すこと（後回し）
- 製品がOpenSpecを検知して、規則を自動で足すこと（ADR-021）

## Decisions

### 追加の許可の組み立て
`settings` に追加の許可の引数を足し、組み込みの許可のあとにつなげる。順番は全体、プロジェクトである。同じ項目が重なったら1つにまとめる。denyの一覧は変えない。計画の実行の `planSettings` は変えない。

`settings` を呼ぶ箇所は、直接の実装、承認のあとの実装、直す実行のすべてで共通である。そのため、追加の許可は引数を1つ渡すだけで、ファイルを書く実行のすべてに届く。

### 追加の許可の検査
`internal/config` の検証で、各項目を次の順に調べる。
1. 前後の空白を除いて空なら誤り
2. `Bash`、`Bash(*)`、`Bash(:*)` なら誤り
3. `Bash(<コマンド>)` の形なら、末尾の ` *`、`:*`、`*` を除いた部分を語に分ける。先頭が `git push`、`git rebase`、`gh` のどれかに一致すれば誤り
4. `WebFetch` で始まれば誤り

`Bash(git *)` のように、拒否している操作を含みうる広い項目は通す。Claude Codeではdenyがallowより優先されるので、拒否している操作は拒否されたままになる。これは検証項目で確かめる。ただし、denyは境界にならない（V5）。フォージの資格情報をClaudeに渡さないこと（ADR-012）が、push対策の本体であることは変わらない。

### 実装用の指示の置き場所
`instructions.Store` に、指示の種別 `Kind`（`plan` と `run`）を足す。`Store.Of(kind)` で種別を選び、`Path`、`Read`、`Save` はファイル名を `plan.md` か `<run.md>` にする。種別の無い `Store` は計画用として動く。ディレクトリの規則と、32 KiBの上限、秘密の検査は共通である。監査ログの `instructions.saved` には `kind` を足す。

### system promptへの入れ方
`PlanSystem` の利用者の追加の指示の節を、共通の関数 `WithUserInstructions` に切り出す。エージェントは、計画の実行以外のsystem prompt（`System`、`ApprovedSystem`、`FixSystem` の結果）に、この関数で実装用の指示を最後の節として足す。指示は実行を始めるたびにファイルから読み直す（計画用と同じ）。

### CLI
`switchboard instructions edit` に `--run` を足す。フラグが無いときは今までどおり計画用を開く。

### 受信箱のAPIとWeb表示
`inboxapi.Instruction` と `SaveInstructionsRequest` に `kind` を足す。GETは、対象ごとに計画用と実装用の2項目を返す。PUTで `kind` が空なら計画用として扱う。古いWeb表示からの保存も、今までどおり計画用の指示に書かれる。未知の `kind` は誤りにする。

Web表示の設定の画面では、対象ごとに「計画の指示」と「実装の指示」の2つの欄を並べ、欄ごとに保存する。追加の許可は、APIとWeb表示のどちらにも出さない。受信箱のトークンや、LANからのアクセスで権限を広げられないようにするためである。

### 文書と記入例
`docs/` に、OpenSpecを使うプロジェクト向けの記入例を置く。中身は、`<allow_tools_extra>` の例（`Skill`、`Bash(openspec *)`）と、`<run.md>` の例である。`<run.md>` の例は次のことを指示する。
- 承認のあとの実装では、最初のコミットで `openspec/changes/<name>/` を作る。issue上の質問と回答を `grilling.md` に写し、承認された計画からproposal、specs、design、tasksを書く
- 計画のモードを通らない直接の実行では、grillingを省いたことを `grilling.md` に記録して、同じように作る
- 人との対話が要るスキルは使わない
- 実装が進むたびにtasksのチェックを付け、終わる前に `openspec validate --strict` を通す

記入例はハーネスの名前を書かず、OpenSpecのCLIと一般的な手順だけで書く（ADR-021）。

## Risks / Trade-offs

- 追加の許可は利用者の責任で広げられる → 読み込みのときの検査で、明らかに危ない項目を誤りにする。denyは境界ではないので、資格情報を渡さない方針（ADR-012、ADR-023）を変えない
- Skillツールで呼んだスキルが、拒否されている操作（`git push`、`gh`）を指示することがある → その呼び出しは拒否されるので、ターンを使うだけで済む。`<run.md>` に、使わないスキルを書いて避ける
- 実装用の指示が組み込みの指示とぶつかる → 節の冒頭に、組み込みの指示を優先すると書く（計画用と同じ）
- 古いWeb表示と新しい `switchboard inbox serve` が混ざる → `kind` が空の保存を計画用として扱う
