# Proposal

## Why

baseとの衝突やCIの失敗を直す実行（fixRun）は順番待ちに入らない。スロットが無いと、次の見張りの確認まで何もしない。衝突だけを見る段では、その確認は `reconcile_interval` ごとになる。一方、実行が終わると、空いたスロットはすぐ順番待ちのissueに渡る。このため、順番待ちが続くかぎり、直す実行はほぼスロットを取れない。作業を終えてマージを待つPR/MRが、衝突したまま止まってしまう（issue #121）。

## What Changes

- スロットを待つ直す実行を、エージェントの内部で覚える。対象は、衝突を直す実行とCIの失敗を直す実行の両方とする。
- スロットが空いたとき、エージェントは順番待ちより先に、待っている直す実行を始める。順は、衝突、CIの失敗、待ち始めた時刻、refの順とする。`agent.prefer_plan` がオンでも、直す実行は計画の実行より先に始まる。
- 直す実行を始める前に、PR/MRとissueを読み直す。衝突が消えた、headが動いた、issueがdoneでなくなったときは始めない。
- 直す実行には順番待ちのコメント（queuedのマーカー）を付けない。スロットを待ち始めたことは、ログに1回だけ残す。
- 設定の項目は足さない。動いている実行は止めない。
- ADR-048を足し、ADR-046の「fixRunは対象外」を補う。DESIGN.mdとREADME.mdの説明を合わせる。

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- `agent-run`: 同時実行の制限に、スロットが空いたとき直す実行を順番待ちより先に始めることを足す。
- `ci-watch`: CIの失敗を直す実行と衝突を直す実行の開始に、スロットが無いときの扱い（順番待ちのコメントを付けず、空いたら先に始める）を足す。

## Impact

- コード：`internal/agent/ciwatch.go` に、見張りの待ちの記録と、待つ直す実行の起動を足す。
- コード：`internal/agent/agent.go` の `drain` と `poll`、`internal/agent/relay.go` の `refresh` から起動を呼ぶ。
- テスト：`internal/agent` の `queue_order_test.go`、`conflict_test.go`、`ciwatch_test.go`。
- 文書：`docs/DECISIONS.md`（ADR-048）、`docs/DESIGN.md`（scheduler）、`README.md`（同時実行の節）。
- 設定、フォージのAPI、トークンの権限、受信箱の表示は変わらない。
