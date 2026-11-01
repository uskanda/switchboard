# Proposal

## Why

`<agent.max_parallel>` は端末全体の上限しか持たない。2026-11-01に、smarternews#68の実行が唯一の枠を使っている間、smarternews#79とswitchboard#40は順番待ちのまま動かなかった。上限を上げるにはエージェントの再起動が要り、再起動すると動いていた#68の実行が中断される。

## What Changes

- プロジェクトの設定に、整数の `<projects[].max_parallel>` を足す。そのプロジェクトで同時に動かす実行の上限になる。無いときは、全体の上限だけで制限する
- `<agent.max_parallel>` は端末全体の上限として残し、既定値の1も変えない
- 順番待ちのissueを取り出すとき、上限に達したプロジェクトのissueを飛ばし、ほかのプロジェクトのissueを実行する
- `<agent.max_parallel>` と `<projects[].max_parallel>` の変更を、設定の読み直しで反映する。上限を下げても、動いている実行は止めない
- プロジェクトの上限が全体の上限を超える設定は、誤りにしない

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

- `agent-run`：同時実行の制限に、プロジェクト単位の上限と、順番待ちの取り出し方を足す
- `config`：`<projects[].max_parallel>` の項目を足し、設定の読み直しで反映する項目に `<agent.max_parallel>` を加える

## Impact

- `internal/config`：`Project` の項目、検証、`Watcher` の変化の扱い
- `internal/agent`：`launch` と `drain` の上限の判定、読み直した上限の受け取り
- `cmd/switchboard`：`Watcher` からエージェントへの受け渡し
- `docs/DESIGN.md` と `README.md` の設定例
