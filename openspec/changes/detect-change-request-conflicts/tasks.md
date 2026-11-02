# Tasks

## 1. フォージから衝突を読む

- [x] 1.1 `forge.ChangeRequestState` に `Conflict` を足し、GitHubのクライアントで `mergeable` と `mergeable_state` から判定する。`false`＋`dirty` だけ真、`null`、`behind`、`blocked` は偽になることを `internal/forge/github` のテストで確かめる
- [x] 1.2 GitLabのクライアントで `has_conflicts` を読み、`detailed_merge_status` が `checking` のときは偽にする。`internal/forge/gitlab` のテストで確かめる
- [x] 1.3 `internal/forge/fake` の変更要求に衝突を設定できるようにし、既存のテストが通ることを `go test ./internal/forge/...` で確かめる

## 2. 見張りを2つの段に分ける

- [x] 2.1 `watch` にCIの段が終わったことと、最後に衝突を問い合わせた時刻を足す。CIが通ったとき、CIが10分現れないとき、6時間で終わらないときに、`unwatch` せず衝突の段へ移す。CIの段の既存のテストに「その後も見張りが残る」確認を足す
- [x] 2.2 `reconcile_interval` ごとに、衝突の段では変更要求だけを読み、CIは読まない。これをテストで確かめる
- [x] 2.3 `findWatch` から6時間の条件を外し、pushから6時間を過ぎたissueを衝突の段で見張りに入れる。再起動のシナリオ（マーカーから衝突の段に戻る）をテストで確かめる
- [x] 2.4 `unwatch` は、変更要求が閉じたとき、headが `sha` と違うとき、issueが `agent:done` でなくなったときに、どちらの段でも起きる。これをテストで確かめる

## 3. 衝突を直す実行

- [x] 3.1 `internal/prompt` に `<FixConflict>` と、その `FixSystem` の分岐を足す。分岐の指示は、`git merge origin/<base>` で解消し、checksを走らせてコミットすることである。矛盾して決められないときは `waiting` で質問させる。プロンプトのテストで文を確かめる
- [x] 3.2 `checkWatch` で、CIを読む前に衝突を見て、衝突を直す実行を始める処理を足す。回数の数え方、枠が無いときの持ち越し、始まった後の `unwatch` は `ciFailed` と同じにする。テストでは、着手のコメントの「baseとの衝突を解消します（1/2）」と、`reason=fix` のマーカーを確かめる
- [x] 3.3 CIの失敗と衝突が同じ問い合わせで見つかったとき、衝突を直す実行が1つだけ始まることをテストで確かめる
- [x] 3.4 headが `sha` と違うときに衝突しても、直す実行が始まらないことをテストで確かめる
- [x] 3.5 `marker` に `conflict` のreasonを足し、上限に達したら `waiting(reason=conflict)` を書く。コメントに変更要求の番号、直した回数、各回の要約があることと、`max_fix_attempts` が0のときも同じになることをテストで確かめる
- [x] 3.6 CIの直しの後で衝突したとき、着手のコメントが「（2/2）」になる（回数の共有）ことをテストで確かめる

## 4. 受信箱とWUI

- [x] 4.1 `web/ui` の `isFailure` に `conflict` を足し、`IssuePane` に返信の案内を出す。`failure.test.tsx` に `conflict` の場合を足して確かめる
- [x] 4.2 `notify` のテストで、通知の理由に `conflict` が載ることを確かめる

## 5. 記録と仕上げ

- [x] 5.1 `docs/DECISIONS.md` にADR-043を足し、`docs/SECURITY.md` の4章に、人のきっかけ無しに始まる実行として衝突を加える。textlintが通ることを確かめる
- [x] 5.2 `make verify` が通ることを確かめる
