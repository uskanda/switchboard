# Tasks

## 1. フォージの層

- [x] 1.1 `forge.ChangeRequestState` に `Computing` を足す。GitHubは `mergeable` が `null`、GitLabは `detailed_merge_status` が `checking` か `unchecked` のときに立てる。両方の `TestChangeRequestConflict` で確かめる
- [x] 1.2 `SetChange` で記録したPR/MRの状態（衝突、計算中、閉じた、マージ済み）を、偽物の `ChangeRequestHead` が返すようにする。`TestChangeRequestHeadReportsASetChange` で確かめる
- [x] 1.3 `forge.CI` のコメントの「受信箱のサービスはCIを見ない」を、ADR-050に合わせて直す

## 2. 受信箱

- [x] 2.1 `Merge` の前提の確かめ方を1つにまとめ、既存のマージのテストが通ることを確かめる
- [x] 2.2 `MergeStatus` を足す。成功、衝突、計算中、headが動いた、マージ済み、チェック無し、CIの権限無し、ほかのCIの誤りを `internal/inbox` のテストで確かめる。doneでない、sha違い、crの無い投稿、空のsha、クライアント無しも確かめる

## 3. API

- [x] 3.1 `inboxapi` に `MergeStatus` の型と、クライアントの `MergeStatus` を足し、クライアントの往復と `stale` の型をテストで確かめる
- [x] 3.2 `inboxserve` に `GET /v1/merge-status` を足す。`CI` の地図があるときだけ出し、応答のJSON、`stale`、`invalid`、`not_found`、権限無しをテストで確かめる
- [x] 3.3 `cmd/switchboard` で、受信箱のトークンのクライアントから `forge.CI` を集めて渡し、`go build ./...` が通ることを確かめる

## 4. WUI

- [x] 4.1 `web/internal/server` に `GET /api/merge-status` の中継を足す。中継、POSTの拒否、誤りの種類の保持、受信箱のサービスに届かないときの `unavailable` をテストで確かめる
- [x] 4.2 `lib/api.ts` に `mergeStatus` と型を足し、`MergeButton` でダイアログを開くたびに読む。`merge.test.tsx` で、読んでいる間、CIの要約と失敗・実行中のリンク、httpでないURL、チェック無し、衝突と計算中を確かめる。headが動いた、権限無し、読めなくてもマージできる、開き直しで読み直すことも確かめる
- [x] 4.3 `DESIGN.md` のマージの項に、マージの前の状態の並びと色を書き足す

## 5. 権限の文書とdoctor

- [x] 5.1 `docs/DECISIONS.md` にADR-050を足す
- [x] 5.2 `docs/SECURITY.md` と `README.md` の受信箱のトークンの権限に、ActionsとCommit statusesの読み取りを足す
- [x] 5.3 `switchboard doctor` で、受信箱のトークンがCIを読めないときに警告する。`TestInboxTokenThatCannotReadCIWarns` で確かめる

## 6. 全体の確かめ

- [x] 6.1 `make verify` が通ることを確かめる
- [x] 6.2 `openspec validate show-merge-status --strict` が通ることを確かめる
