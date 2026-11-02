# Proposal

## Why

計画を先に立てるモードでは、計画の実行は質問と返答を何度も繰り返すので、早く始めたい。実装の実行は、後でまとめて動かしても困らない。
これまでのエージェントは、空いたスロットを順番待ちのissueにrefの文字列の辞書順で渡していた。計画か実装かは見ず、`#10` が `#9` より先に動き、待った時間とも関係が無かった（issue #74）。

## What Changes

- 端末の設定 `agent.prefer_plan`（bool、既定はfalse）を足す。
- `agent.prefer_plan` がオンのとき、エージェントは、順番待ちのうち計画の実行を先に取り出す。承認済みの計画の実装、計画なしの実装、返答による再開、doneからの再開は後に回す。
- 同じ種類の中では、順番待ちへ入った順でスロットを渡す。設定がオフのときも同じである。同じ時刻ならrefの文字列の順にする。refの辞書順はやめる。
- 順番待ちのあいだに実行の種類が変わっても（計画が承認されたときなど）、入った時刻は変えない。
- 並べ替えるのは順番待ちから取り出すときだけである。スロットが空いていれば、実装もすぐに始まる。
- 設定の読み直しで反映する項目に `agent.prefer_plan` を加える。再起動は要らない。
- ADR-046で、取り出す順と `agent.prefer_plan` を決め、ADR-039が読み直す範囲を広げる。

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

- `config`: `agent.prefer_plan` の設定を足し、読み直しで反映する項目に加える
- `agent-run`: 順番待ちから取り出す順を、入った順と、`agent.prefer_plan` による計画の実行の優先に決める

## Impact

- `internal/config`：`Agent` に `PreferPlan` を足し、`Change` に載せて読み直しで反映する
- `internal/agent/agent.go`：順番待ちの各issueに、計画の実行かどうかと入った時刻を持たせ、取り出す順を変える
- `internal/agent/reload.go`：読み直した `agent.prefer_plan` を `agent.max_parallel` と一緒に反映する
- `docs/DECISIONS.md`：ADR-046を足す
- `README.md`、`docs/DESIGN.md`：設定の説明と取り出す順を書き足す
- 不変条件は変わらない
