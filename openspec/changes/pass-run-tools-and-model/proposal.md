# Proposal

## Why

`claude -p` はCLIの既定のツールを約25個受け取り、起動直後の文脈が約39kトークンになる。モデルとeffortもユーザーの全体設定（`opus[1m]` と `xhigh`）をそのまま引き継ぐ。177回の実行で使ったツールは、Bash、Read、Edit、Write、StructuredOutputがほぼすべてだった。渡すツールを絞り、モデルとeffortをswitchboardの設定で選べるようにして、トークンと処理時間を減らす（#129）。

## What Changes

- 実行に渡すツールを固定のリストにする。実装の実行と直す実行には `Bash,Read,Edit,Write,Glob,Grep,WebSearch` を渡し、計画の実行には `Bash,Read,Glob,Grep,WebSearch` を渡す。
- `WebSearch` を、どの実行でも許可する。
- `<agent.allow_tools_extra>` と `<projects[].allow_tools_extra>` の各項目のツール名を、渡すツールに足す。
- 設定に、Claude Code固有でない名前のモデルとeffortの項目を足す。`<agent.model>` と `<agent.effort>` が既定で、`<agent.plan>` と `<agent.fix>` で上書きできる。空のときは、CLIの既定に任せる。
- effortの値を、設定の読み込みのときに検査する。

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

- `agent-run`: 実行に渡すツール、モデル、effortの要件を足す。追加の許可がツールも足すことを書き足す。
- `config`: モデルとeffortの設定と、その検査を足す。
- `plan-mode`: 計画の実行に渡すツールを書き足す。

## Impact

- コード：`internal/claudecode`（起動の引数）、`internal/agent`（ツール、モデル、effortの決定）、`internal/config`（項目と検査）。
- 文書：`docs/DECISIONS.md` にADRを足し、`docs/ROADMAP.md` の検証項目に `--tools` と `--effort` の確認結果を書く。設定の例と `docs/ARCHITECTURE.md` の起動の説明を直す。
- 既存の設定は、新しい項目が無くても今までどおり読める。モデルとeffortの項目が無いと、今までと同じくユーザーの設定が使われる。
