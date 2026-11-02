# Proposal

## Why

マージの確認ダイアログには、取り消せないことの1文とPR/MRのURLしかない。CIが通っているか、衝突が無いかを確かめるには、フォージを開く必要があった（issue #84）。

## What Changes

- 受信箱のサービスに、完了のissueのPR/MRの状態を読む口 `GET /v1/merge-status` を足す。PR/MRの状態、headが画面のコミットから動いたか、衝突（なし／あり／フォージが計算中）、コミットのチェックの一覧を返す。前提はマージと同じで、外れれば `stale` を返す
- WUIに `GET /api/merge-status` を足し、受信箱のサービスへ中継する
- WUIのマージの確認ダイアログは、開くたびに状態を読む。衝突、headが動いたこと、CIの要約、失敗中と実行中のチェックへのリンクを出す。表示は情報だけで、「マージする」は押せるままにする
- ADR-047を足し、ADR-043の「受信箱のサービスはCIを見ない」をこの点だけ改める。GitHubの受信箱のトークンに `Actions: Read` と `Commit statuses: Read` を足す。権限が無いときは、ダイアログにCIを読めないことを出す
- `switchboard doctor` は、受信箱のトークンでCIを読めないときに警告する

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

- `inbox-api`: マージの前の状態の読み出しの口を足す
- `web`: マージの確認のダイアログに、マージの前の状態を出す。状態の読み出しの中継を足す

## Impact

- コード：受信箱の側は `internal/forge`、`internal/inbox`、`internal/inboxserve`、`inboxapi` を変える。`cmd/switchboard` と `internal/doctor` も直す。WUIの側は `web/internal/server` と `web/ui` を変える
- API：受信箱のサービスに `GET /v1/merge-status`、WUIに `GET /api/merge-status` が増える。一覧の要素は変えない
- 権限：GitHubの受信箱のトークンに、ActionsとCommit statusesの読み取りが要る。足さなくても、ほかの機能は変わらない
- 文書：`docs/DECISIONS.md`（ADR-047）、`docs/SECURITY.md`、`README.md`、`DESIGN.md`
