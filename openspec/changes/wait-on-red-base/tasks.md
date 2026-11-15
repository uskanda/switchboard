# Tasks

## 1. フォージとマーカー

- [x] 1.1 `internal/forge` の `CI` に `BranchHead` を足し、GitHubとGitLabのアダプタで実装する（design.mdのD2）。各アダプタのテストで、偽のサーバーが返すブランチの先頭のコミットを読み、存在しないブランチで `ErrNotFound` になることを確かめる
- [x] 1.2 `internal/marker` に状態 `note` と `fix` の値 `base` を足し、`note` を `Transient()` に含める（D4）。`marker` のテストで、`state=note reason=base` を書いて読み戻せることと、`Transient()` が真になることを確かめる
- [x] 1.3 `internal/inbox` に失敗するテストを書く。最新のdoneの投稿のあとに `state=note reason=base` のコメントがあっても、issueが完了として一覧に出ることを確かめる。1.2のあとで通ることを確かめる

## 2. baseが赤の判定と待ち

- [x] 2.1 `internal/agent` に失敗するテストを書く。PRのCIでjob `verify` が失敗し、baseの先頭でも `verify` が失敗している状態を作る。直す実行が始まらないこと、回数が増えないこと、issueが `agent:done` のままであることを確かめる（spec「baseが赤の判定」「baseが赤の間の待ち」）
- [x] 2.2 `internal/agent` に失敗するテストを書く。PRだけが別のjobでも落ちるとき、baseにCIが無いときは、今までどおり直す実行が始まることを確かめる。baseのCIが動いている間は、直す実行が始まらないことも確かめる
- [x] 2.3 `checkWatch` と `ciFailed` に、baseの状態を引く処理と `watch` の `baseWait` を足す（D1）。2.1と2.2のテストと、既存の `ciwatch_test.go`、`fix_test.go` が通ることを確かめる
- [x] 2.4 `internal/agent` に失敗するテストを書く。10件のissueが同じプロジェクトのbaseで待つとき、1回の問い合わせで `BranchHead` とbaseの `CommitChecks` がそれぞれ1回だけ呼ばれることを確かめる。API制限の間は呼ばれないことも確かめる。組ごとのメモ（D2）を足して通す
- [x] 2.5 `internal/agent` に失敗するテストを書く。待ち始めに `state=note reason=base` のコメントが1回だけ付き、10回の問い合わせと、エージェントの起動し直しのあとも増えないことを確かめる（spec「待ち始めのコメント」）。`messages.go` へ日本語と英語の文言を足して通す
- [x] 2.6 `internal/agent` に失敗するテストを書く。baseが赤の間にPRが衝突しても衝突を直す実行が始まらず、緑に戻ると始まることを確かめる（spec「baseが赤の間の衝突」）。`conflicted` の前でbaseを判定する。`conflictsOnly` の段でも、`baseWait` の間は `poll_interval` ごとに見る。この2つを足して通す

## 3. 緑に戻ったときの取り込み

- [x] 3.1 `internal/workspace` に、baseを `git merge --no-edit` で取り込み、衝突したら `git merge --abort` で戻す処理を足す。一時的なgitのリポジトリを使うテストで、取り込めた場合、衝突した場合、取り込むものが無い場合の3つを確かめる
- [x] 3.2 `internal/agent` に失敗するテストを書く。baseが緑に戻ると、Claudeを起動せずに取り込んでpushし、`fix=base` と新しい `sha` のdoneを投稿し、新しい `sha` を見張ることを確かめる。回数が変わらないこと、ラベルが `agent:done` のままであること、着手のコメントが無いことも確かめる（spec「baseが緑に戻ったときの取り込み」）
- [x] 3.3 `internal/agent` に失敗するテストを書く。取り込みで衝突したら衝突を直す実行が始まり、回数に数えることを確かめる。worktreeのheadが `watch` の `sha` と違えば、取り込まずに見張りをやめることも確かめる
- [x] 3.4 design.mdのD3の手順で取り込みを実装し、監査ログの4つの出来事を残す。3.2と3.3のテストと、`go test ./internal/agent ./internal/workspace` が通ることを確かめる

## 4. 自動マージの猶予

- [x] 4.1 `internal/inbox` に失敗するテストを書く。自動マージの依頼のあと、PRのheadがマーカーの `sha` と違っても、2分以内に `fix=base` のdoneが追いつけば依頼が続き、その `sha` でマージされることを確かめる。2分を過ぎても違えば `ReasonPushed` で打ち切ることも確かめる
- [x] 4.2 `internal/automerge` の依頼に、headの食い違いを初めて見た時刻を足し、`internal/inbox/automerge.go` の判定に猶予を入れる（D5）。4.1のテストと、既存の `automerge_test.go` が通ることを確かめる

## 5. マージのダイアログのbaseのCI

- [x] 5.1 `internal/inboxserve` に失敗するテストを書く。mainの先頭でjob `verify` が失敗しているとき、マージの前の状態に `base_branch` と `base_failed` が載ることを確かめる。mainが緑なら空、baseを読めないときはほかの欄が今までどおり返ることも確かめる（inbox-apiのspec）
- [x] 5.2 `inboxapi.MergeStatus` に `base_branch` と `base_failed` を足し、`merge_status.go` で埋める（D6）。5.1のテストと、`web/internal/server` の `merge_status_test.go` が通ることを確かめる
- [x] 5.3 `web/ui` に失敗するテストを書く。`base_failed` があるときだけダイアログにbaseのCIの行が出て、「マージする」が押せることを確かめる。英語の表示も確かめる（`merge.test.tsx`、`englishMerge.test.tsx`）。`MergeButton.tsx` の `defineMessages` に日本語と英語の文言を足し、`web/ui/src/lib/api.ts` の型を直して通す

## 6. 記録と文書

- [x] 6.1 `docs/DECISIONS.md` にADR-075を足す。決めたこと（baseが赤の判定、待ち、緑に戻ったときの取り込み、一時的なマーカー、自動マージの猶予）を書く。理由として2026-11-15のsmarternewsの件も書く。ADR-041とADR-044を補うことも書く。ADR-074と同じ形であることを読んで確かめる
- [x] 6.2 `docs/ROADMAP.md` の検証項目へV67を足す。smarternewsとswitchboardのCIで、`push` と `pull_request` のチェックの名前が揃うことを、`gh api` で読んだ結果とともに書く
- [x] 6.3 `docs/ARCHITECTURE.md` のCIの見張りの節（4.4と4.5）に、baseが赤のときの待ちと取り込みを書く。`docs/GUIDE.md` と `docs/GUIDE.ja.md` のCIの直しの節に、同じ変更で同じ内容を書く。両方の版を読み比べて確かめる
- [x] 6.4 `make verify` を、sandboxの外で実行して通ることを確かめる

## 7. 本番での確認

- [ ] 7.1 `switchboard agent run` と受信箱のサービスを、新しいバイナリで起動し直す。smarternewsのmainが赤のとき、新しく失敗したPRに待ち始めのコメントが1つだけ付き、回数が増えないことを、issueと監査ログで確かめる。mainが緑に戻ったあと、`<ci.base.merged>` が残り、PRのCIが走り直すことを確かめる
