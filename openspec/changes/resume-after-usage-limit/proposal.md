# Proposal

## Why

サブスクリプションの利用上限に当たった実行は、今は「claudeがエラーで終わりました：success」という `waiting(reason=error)` になる。人が解除時刻を待って返信しない限り、作業が止まったままになる。上限は時間が経てば解ける。人の手を借りずに、解除後に自動で再開できるようにしたい。API利用では、費用の上限（クオータ）も設けたい。

## What Changes

- 実行の結果に、サービスに依存しない「上限に当たった」型（種類、解除予定の時刻、メッセージ）と費用を足す。Claude Codeのアダプタが `rate_limit_event` のrejectedとresultの429から埋める
- 上限で止まった実行のissueを、`agent:ready` と順番待ちのコメント（`state=queued reason=limit`）に戻す。解除予定の時刻を過ぎたら自動で再開し、そのコメントを消す
- 再開した実行のプロンプトには、前の実行が上限で止まったことを足す。計画の実行には足さない
- 上限に当たった端末は、解除予定の時刻まで新しい実行を始めない。その間に始まるはずだったissueは、`reason=limit` の順番待ちにする。止めた状態はファイルに書き、再起動しても引き継ぐ
- `claude.auth: api_key` のとき、各実行の費用を端末で積み上げる。`quota.daily_usd` か `quota.monthly_usd` を超えたら、日か月の切り替わりまで新しい実行を始めない。順番待ちの理由は `quota` とする
- 直す実行が上限に当たったときは、端末を止めたうえで `waiting(reason=error)` で終え、自動で直す回数に数えない
- 設定に `agent.limit_wait` と `quota` を足す
- 受信箱（CLIとWeb）の順番待ちの節に、理由が `limit` か `quota` のとき再開の時刻を出す
- DESIGN.mdの `waiting(reason=limit)` を、順番待ちとしての扱いに置き換える（ADR-047）

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

- `agent-run`: 上限で止まった実行の順番待ちへの戻し、端末の停止、クオータ、再開した実行のプロンプト。順番待ちのコメントの理由と削除
- `run-outcome`: 上限で止まった実行を異常な終了から外す。直す実行が上限に当たったときの扱い、マーカーの `until`
- `config`: `agent.limit_wait` と `quota` の設定と検査
- `inbox`: 順番待ちの行の再開の時刻
- `inbox-api`: 一覧の項目の `queued_until`
- `web`: 順番待ちの行の再開の時刻

## Impact

- 実行の結果の型：`internal/usage`（新規）、`internal/claudecode`
- エージェントと設定：`internal/agent`、`internal/config`、`internal/marker`、`internal/doctor`
- 受信箱：`internal/inbox`、`inboxapi`、`web/ui`
- 偽フォージ（`internal/forge/fake`）の `SetLabels` が、足したラベルの付与者をエージェントにする（実際のフォージと同じにする）
- 状態ディレクトリに `pause.json` と `cost.json` が増える
- 文書：`docs/DECISIONS.md`（ADR-047）、`docs/DESIGN.md`、`docs/ROADMAP.md`（V12）、README.mdの設定の説明
