# Tasks

## 1. doneのマーカーの `fix`

- [x] 1.1 `internal/marker` に `fix=` の書き出しと読み取り、無い場合のテストを足し、落ちることを確かめる。`Marker` に `Fix` を足してテストを通す
- [x] 1.2 `internal/agent` に、直す実行のdoneのマーカーに `fix` が付き、ほかの実行には付かないことのテストを足し、落ちることを確かめる。`outcome` と `writeOut` を直してテストを通す

## 2. フォージの層

- [x] 2.1 `forge.Merges` に `ChangeRequestHead` を足す。fakeの `Change` に `Conflict` と `Closed` を足し、`SetChange` の状態を `ChangeRequestHead` が返すテストを書いてから直す

## 3. 依頼の置き場

- [x] 3.1 `internal/automerge` の `Store` のテストを書き、落ちることを確かめてから実装する。テストでは、保存と読み直し、置き換え、取り消しを確かめる。終わった依頼が24時間で消えることも確かめる

## 4. 受信箱の判断と見張り

- [x] 4.1 `internal/inbox` の`AutoMergeCandidate` と `AutoMergeStep` のテストを、判断の順の分岐ごとに書き、落ちることを確かめてから実装する
- [x] 4.2 `inboxserve` のテストを書き、落ちることを確かめてから実装する。対象は、マージのPOSTの202と取り消しのPOST、一覧と詳細の `auto_merge` と `auto_merge_results`、購読が無くても試す見張りである
- [x] 4.3 `inboxapi` の型とクライアント（`Merge` の戻り値、取り消し）を直し、テストを足す
- [x] 4.4 `cmd/switchboard` の `inbox serve` で `Store` を開き、見張りを動かす

## 5. WUIの裏側

- [x] 5.1 `web/internal/server` に `/api/automerge/cancel` を足し、`/api/merge` の202を返す。検査（`Origin` の違い、JSONでない本文）のテストを先に書く

## 6. WUI

- [x] 6.1 `merge.test.tsx` に、202のトーストと印、取り消し、結果のトーストのテストを足し、落ちることを確かめる
- [x] 6.2 `lib/api.ts`、`MergeButton`、`InboxList`、`IssuePane`、結果のトーストを実装し、テストを通す

## 7. 文書

- [x] 7.1 `docs/DECISIONS.md` にADR-047を足す
- [x] 7.2 `docs/ARCHITECTURE.md`、`docs/SPEC.md`、`docs/ROADMAP.md`、`README.md`、`DESIGN.md` を直す

## 8. 全体の確認

- [x] 8.1 `make verify` を通し、`openspec validate auto-merge-after-conflict-fix --strict` を通す
