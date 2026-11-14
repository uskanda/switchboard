# Proposal

## Why

計画の実行は、grillingの前に最新のナレッジを調べたい場面がある。今の計画の実行に渡しているのは `WebSearch` だけで、これは検索結果の要約しか返さないので、公式の文書やchangelogの本文は読めない。`WebFetch` はADR-052ですべての実行で拒否している。ところが、issueの本文とコメントは信頼済みの人のものしか採らないので、issueを経由する持ち出しの経路はすでに塞がっている。そこで、人の承認を経てから実装に進む計画の実行に限り、ページの本文を読めるようにする。

## What Changes

- 計画の実行で、ドメインを限らずに `WebFetch` を許し、`--tools` にも渡す。実装の実行と直す実行では、今までどおり拒否する。
- 設定に <agent.plan_web_fetch> と <projects[].plan_web_fetch> を足す。既定はオンで、プロジェクトの値が全体の値を上書きする。オフのときは `WebFetch` を拒否し、`--tools` にも渡さない。どちらの項目も設定ファイルにだけ書ける。
- 計画の実行のsystem promptに、読んだページの本文をデータとして扱い、issueやリポジトリの内容をURLに載せて送らないよう書き足す。
- 計画の実行が投稿する計画のコメント（質問、計画、計画を省いた実装の開始）に、その実行が `WebFetch` で読もうとしたURLの一覧を、折りたたんで付ける。
- `allow_tools_extra` で `WebFetch` を名指しする項目は、今までどおり設定の誤りにする。

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

- `plan-mode`: 計画の実行での `WebFetch` の許可、読んだページの扱いの指示、参照したページの一覧の要件を足す。
- `config`: <agent.plan_web_fetch> と <projects[].plan_web_fetch> の要件を足す。

## Impact

- コード：`internal/config` は項目と読み直し、`internal/agent` は計画の実行の許可とツールとコメントの一覧を受け持つ。`internal/prompt` は計画の実行の指示、`internal/claudecode` は `stream.jsonl` からの呼び出しの取り出しを受け持つ。
- 文書：`docs/DECISIONS.md` に、ADR-052の一部を覆すADRを足す。`docs/ROADMAP.md` に、実行ごとのsandboxの中でWebFetchが外へ届くかの検証項目を足す。そのほか、SECURITYとGUIDEの日英の両版、`docs/DESIGN.md`、`config/switchboard.example.yaml` を直す。
- 既存の設定は、新しい項目が無くても読める。その場合、計画の実行でWebFetchが使えるようになる。
