# Tasks

## 1. 実行の結果の型とアダプタ（`internal/usage`、`internal/claudecode`）

- [x] 1.1 `internal/usage` に `LimitHit` を置き、`claudecode.Result` に `Limit` と `CostUSD` を足す（design D1）。`testdata/limit.jsonl` から解除時刻と種類を読むことをテストで確かめる。429だけの出力では解除時刻が無いこと、成功のresultでは上限にしないこと、`total_cost_usd` を読むことも確かめる

## 2. 設定（`internal/config`、`internal/doctor`）

- [x] 2.1 `agent.limit_wait`（既定30m）と `quota` を足し、負の値を誤りにする。読み込み、既定値、誤りをテストで確かめる
- [x] 2.2 `subscription` で `quota` を書いたときの警告を `Warnings` で返し、`switchboard doctor` が出す。テストで確かめる

## 3. マーカー（`internal/marker`）

- [x] 3.1 順番待ちの理由 `limit` と `quota`、キー `until` を足す。書き出しと読み取りの往復をテストで確かめる

## 4. エージェント（`internal/agent`、偽フォージ）

- [x] 4.1 偽フォージの `SetLabels` が、足したラベルの付与者を投稿のユーザーにする（design D8）。既存のテストが通ることで確かめる
- [x] 4.2 上限の結果を順番待ちのコメントとして書き戻し、`agent:ready` に戻す（design D2）。ラベル、コメント1つ、通知なしをテストで確かめる
- [x] 4.3 端末の停止（`pause.json`、`hasSlot`、タイマー）を足す（design D4）。停止中の別のissueが `reason=limit` の順番待ちになり、動いている実行は止めないことをテストで確かめる。時刻を進めると両方が始まってコメントが消えることも確かめる。再起動しても止まったままなこと、解除時刻が無いとき `limit_wait` の後に始まることも確かめる
- [x] 4.4 上限からの再開（design D3）を足す。エージェントが付けたラベルで再開すること、再開のプロンプトに一文が入ること、計画の実行には入らないこと、承認済みの計画の実装として再開することをテストで確かめる
- [x] 4.5 クオータ（design D5）を足す。`daily_usd` の超過で `reason=quota` の順番待ちになり、日付が変わると始まること、`subscription` では数えないことをテストで確かめる
- [x] 4.6 直す実行の上限（design D6）を足す。`waiting(reason=error)` になり、自動で直す回数が増えないことをテストで確かめる

## 5. 受信箱（`internal/inbox`、`inboxapi`、`web/ui`）

- [x] 5.1 `queued_until` を足し、CLIの順番待ちの節に再開の時刻を出す。テストで確かめる
- [x] 5.2 WUIの順番待ちの行に、利用上限とクオータの文と再開の時刻を出す。vitestで確かめる

## 6. 文書

- [x] 6.1 ADR-053を `docs/DECISIONS.md` に足し、`docs/DESIGN.md` の状態遷移表・停止の表・マーカーの表を直す
- [x] 6.2 `docs/ROADMAP.md` のV12に、確かめた出力の形を記録する。README.mdの設定の説明に `agent.limit_wait` と `quota` を足す

## 7. 確かめ

- [x] 7.1 `make verify` と `openspec validate resume-after-usage-limit --strict` を通す
- [ ] 7.2 本番で次に上限に当たったとき、自動で再開したことをV12に書き足す
