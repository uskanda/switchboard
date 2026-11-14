# Design

## Context

計画の実行の許可は `planSettings`（`internal/agent/planrun.go`）が組み立て、denyに `WebFetch` を固定で入れている。渡すツールは `planToolsFor` が `planTools` から作る。実装の実行は `run.go` のdenyに `WebFetch` を入れている。設定の読み込みは、`allow_tools_extra` に `WebFetch` を書くと誤りにする（`internal/config/config.go`）。

計画のコメントは `planQuestions`、`planText`、`planSkip` が組み立て、末尾は `planTail` が作る。`planTail` は、拒否された呼び出しの一覧をすでに `<details>` で付けている。各実行の出力は `RunDir/stream.jsonl` に残り、終わったあと `internal/claudecode` の `Parse` が読む。

動機はproposal.mdのWhyを参照。決まったことはgrilling.mdにある。

## Goals / Non-Goals

**Goals:**
- 計画の実行だけで、ドメインを限らずにページの本文を読めるようにする。
- 読んだURLを、計画を承認する人がコメントの上で見られるようにする。

**Non-Goals:**
- 実装の実行と直す実行でのWebFetch。
- ドメインの許可リスト、監査ログへの記録。
- 受信箱の画面からの切り替え。

## Decisions

### D1 設定の値は `*bool` で持ち、解決は設定の側に置く

`config.Agent` と `config.Project` に `PlanWebFetch *bool`（`yaml:"plan_web_fetch"`）を足す。プロジェクト、全体、既定のオンの順に解決するメソッドを `config` に置き、`internal/agent` はその結果だけを使う。`*bool` にするのは、「書いていない」と「`false` と書いた」を分けるためである。プロジェクトの項目は、`allow_tools_extra` と同じく `projects` の読み直しで変わる。全体の項目は起動時に読むだけにする。

### D2 許可とツールは引数1つで切り替える

`planSettings` と `planToolsFor` に、WebFetchを許すかどうかの真偽値を足す。オンなら、allowに `WebFetch` を足し、denyから外し、ツールに `WebFetch` を足す。オフなら今と同じ結果にする。実装の実行の `run.go` は変えない。`allow_tools_extra` の `WebFetch` の検査も変えないので、計画の実行のWebFetchを開く経路は新しい項目だけになる。

ドメインを限らない許可は、規則の `WebFetch` 1つで書ける（`WebFetch(domain:…)` は使わない）。

### D3 指示は、あとから節を差し込む関数で足す

`prompt.WithPlanImages` と同じ形の `prompt.WithWebFetch` を作り、オンのときだけ計画の実行のsystem promptの「Rules」の節の前に差し込む。`prompt.PlanSystem` と `prompt.PlanSkipSystem` の引数は変えない。差し込む節には、読んだページの本文をデータとして扱うこと、issueやコメントやリポジトリの内容をURLに載せないことを書く。文言は英語で書き、ほかの指示の節と同じ形にする。

### D4 URLは `stream.jsonl` のtool_useから取る

`internal/claudecode` の `Parse` が、名前が `WebFetch` の `tool_use` を、assistantのメッセージから探す。その入力の `url` を集めて `Result` に入れる。拒否された呼び出しも `tool_use` として出力に出るので、拒否と失敗を含めた全部が取れる。重複をまとめて最初の順を保つのも、ここで行う。Claude Codeの出力の形に依存する部分なので、アダプタの `claudecode` に閉じ込める（AGENTS.mdの方針）。

あとから `stream.jsonl` を読み直す案は採らない。実行の終わりに `Parse` がすでに全体を読んでいるからである。

### D5 一覧は `planNotes` で、拒否の一覧の前に付ける

拒否の一覧と捨てた変更の行を作る部分を `planNotes` に切り出し、集めたURLを渡して、`<details>` の折りたたみで付ける。見出しの文言は日本語と英語の両方を `msg` に持つ。各URLは、200文字で打ち切り、末尾に `…` を付けてコード表記にする。21件目以降は「ほかN件」の1行にする。`planQuestions`、`planText`、`planSkip` の3つとも `planNotes` を通るので、質問、計画、計画を省いた実装の開始のどれにも付く。

URLの中にバッククォートがあっても表記が崩れないよう、URLに含まれないだけの長さのバッククォートで囲む。

## Risks / Trade-offs

- リスク：読んだページに仕込まれた文が、モデルに別のURLへ情報を送らせる。対策：D3の指示で抑え、D5の一覧で人が見られるようにする。計画は人の承認を経てから実装に進み、Claudeの環境に資格情報は無い（ADR-012）。秘密のコードのプロジェクトは <projects[].plan_web_fetch> で外せる。
- リスク：実行ごとのsandboxや、ユーザーの設定がWebFetchを止めて、許しても使えない。対策：実装の前にROADMAP.mdの検証項目（V66）として、実物の `claude -p` で確かめる。C20によれば、sandboxのネットワークの制限はWebFetchに及ばない。
- リスク：`tool_use` の入力の形が変わってURLが取れない。対策：`claudecode` のテストに実物の出力の断片を置き、取れないときは一覧を付けないだけで、実行は失敗にしない。
- リスク：ページの本文が文脈を増やし、計画の実行が長くなる。対策：WebFetchは要約を返す作りで、使うかどうかはモデルの判断に任せる。計画の実行には上限の時間がすでにある。

## Migration Plan

新しい項目が無い既存の設定は、そのまま読める。その場合、計画の実行でWebFetchが使えるようになる。戻すときは、<agent.plan_web_fetch> を `false` にすれば今までと同じ動きになる。

## Open Questions

なし。
