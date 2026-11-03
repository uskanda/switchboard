# Design

## Context

`internal/claudecode` の `Args` は、ツール、モデル、effortを指定せずに `claude -p` を起動している。そのため、CLIの既定のツールと、ユーザーの全体設定のモデルとeffortが使われる。許可の規則は、実行の種類ごとに `settings` と `planSettings` が作る。`dontAsk` では許可の規則が無いツールは拒否され、`--tools` に無いツールはセッションに渡らない。このため、渡すツールと許可の規則の両方を揃える必要がある。

実測の結果（claude 2.1.316）は次のとおり。

- `--tools "Bash,Read,Edit,Write,Glob,Grep"` で、起動直後の文脈が約39kから約9kトークンになった。`--json-schema` を付けると、構造化出力のツールは自動で足され、構造化出力は動いた。
- `--model sonnet` はinitの `model` に反映された。
- `--effort` は受け付けられるが、initとresultに値は出ない。

## Goals / Non-Goals

**Goals:**

- 実行の種類ごとに、渡すツール、モデル、effortをswitchboardが決める。
- 設定の名前をClaude Code固有にしない。

**Non-Goals:**

- `--setting-sources` によるユーザー設定の切り離しと、sandboxの扱い（#130）。
- 2つ目のエンジンのためのインターフェース。
- プロジェクトごとのモデルとeffortの上書き。

## Decisions

### ツールの決定は `agent` が持ち、`claudecode` は渡すだけにする

`claudecode.Request` に `Tools []string`、`Model string`、`Effort string` を足す。`Args` は、`Tools` を `--tools` にカンマで連結して渡す。`Model` と `Effort` は、空でないときだけ `--model` と `--effort` を付ける。どのツールを渡すかは、許可の規則と同じく `internal/agent` が決める。許可の規則を作る関数の隣に置けば、2つの一覧がずれにくい。

### 追加の許可からツール名を取り出す

`allow_tools_extra` の各項目から、最初の `(` より前をツール名として取り出す。`Bash(openspec *)` は `Bash`、`Skill` は `Skill` になる。取り出したツール名を、既定のツールの後ろに、重複を除いて足す。別の項目 `tools_extra` を作る案は採らない。利用者が、許可とツールの両方を書くことになるためである（grilling.mdの第3ラウンドQ9）。

### 計画の実行の `WebSearch` と拒否の規則

計画の実行の許可に `WebSearch` を足す。計画の実行のツールに `Edit`、`Write`、`NotebookEdit` は無い。それでも、`planSettings` のdenyは残す。将来ツールの一覧が変わったときにも、書き込みを止めるためである。

### モデルとeffortの解決

`config.Agent` に `Model`、`Effort`、`Plan`、`Fix` を足す。`Plan` と `Fix` は、`Model` と `Effort` を持つ同じ型にする。実行の種類から上書きを選び、空の項目を既定で埋める処理は、設定の側に置く。`internal/agent` は、自分の実行の種類を設定の側の種類に対応させるだけにする。承認のあとの実装の実行と衝突を直す実行は、それぞれ実装の実行と直す実行に含める。

### effortの許容値はアダプタが持つ

許容値の一覧は、<claudecode.Efforts> としてアダプタに置く。`config` の検査はこの一覧を参照する。CLIが値を増やしたときに直すのは、アダプタの1か所になる。

## Risks / Trade-offs

- リスク：`--tools` で渡さないツールを、モデルが使えなくなる。対策：177回の実行で、ほかのツールの利用はAgentの3回とTaskStopなどの数回だけだった。必要なツールは `allow_tools_extra` に書けば渡る。
- リスク：`--effort` の値が実行に適用されたかを、initで確かめられない。対策：ROADMAPの検証項目に書き、実行時間と出力のトークンの変化で確かめる。
- リスク：WebSearchの検索語に、作業中の情報が入る。対策：WebSearchはページの本文を取ってこないため、WebFetchより出入りする情報が少ない。フォージの資格情報は渡さない（ADR-012）ので、検索語に秘密が載ることは無い。ADRに記録する。

## Migration Plan

新しい項目が無い設定は、今までどおり読める。モデルとeffortは、項目を書いたときだけ変わる。ツールの絞り込みは、更新したバイナリで再起動したあとの実行から適用される。戻すときは、前のバイナリに戻す。
