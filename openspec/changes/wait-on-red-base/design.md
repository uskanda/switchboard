# Design

## Context

動機はproposal.mdのWhyを参照する。決定の出典はgrilling.mdである。

今のCIの見張り（`internal/agent/ciwatch.go`）は、issueごとの `watch` を持つ。PR/MRのheadのCIだけを読み、失敗すれば `ciFailed` が直す実行を始める。回数が上限を超えていれば、`waiting(reason=ci)` を書く。CIの段が終わると、`conflictsOnly` の段で `reconcile_interval` ごとに衝突だけを見る。衝突を見つけると `conflicted` が衝突を直す実行を始める。baseのブランチのCIは、どこでも読んでいない。

受信箱の自動マージ（`internal/inbox/automerge.go`、ADR-048）は、エージェントの最新のdoneのマーカーの `fix` が空でなければ受け付ける。ただし、PR/MRのheadがそのマーカーの `sha` と違うと、「人がpushした」として依頼を打ち切る。今の直す実行は、その間issueが `agent:running` なので、この判定の前で止まる。

## Goals / Non-Goals

**Goals:**

- baseのCIの失敗を、PR/MRの失敗と取り違えない。
- baseが緑に戻ったら、人の操作なしにPR/MRを動ける状態に戻す。
- 待っているPR/MRが何件あっても、baseの読み取りは組ごとに1回で済ませる。

**Non-Goals:**

- 受信箱からのマージの直列化と、赤い間のマージの保留（`serialize-inbox-merges`）。
- PR/MRのCIの再実行（`Actions: Write` が要る）。
- すでに `waiting(reason=ci)` のissueの移行（grilling.md Q9）。

## Decisions

### D1. baseの判定はCIの段と衝突の段の両方に挟む

`checkWatch` は、CIの失敗を `ciFailed` へ渡す前と、衝突を `conflicted` へ渡す前に、baseの状態を引く。状態は `green`、`red`、`pending` の3つとする。`red` は失敗したチェックの名前の集合を持つ。

- CIの失敗では、PR/MRで失敗した名前がすべて `red` の集合に入っているときだけ待つ（spec「baseが赤の判定」）。`pending` は待ち、次の問い合わせで引き直す。
- 衝突では、baseが `red` なら待つ（spec「baseが赤の間の衝突」）。ここでは名前の突き合わせはしない。赤いbaseを取り込む直しは、どれも落ちるためである。

待っている `watch` には `baseWait` の印を付ける。`conflictsOnly` の段でも、印のある間は `reconcile_interval` でなく `poll_interval` ごとに見る。

代わりの案：PR/MRのCIの失敗をすべて、baseと関係なく1回だけ数えない案。PR/MR自身の誤りでも1回の実行が遅れる。baseが赤い間は、数えない失敗を延々と繰り返す。名前を突き合わせれば、対象を絞れる。

### D2. baseの読み取りは、問い合わせの周期ごとに組でまとめる

`Agent` に、プロジェクトとbaseのブランチの組をキーにしたメモを持つ。値は、読んだ時刻、先頭のコミット、状態である。`poll_interval` の周期の中で2件目以降の `watch` は、メモを使う（spec「baseの読み取りの間隔」）。API制限の間（ADR-072）は読まず、メモも更新しない。

フォージの `CI` へ `BranchHead(ctx, owner, repo, branch) (string, error)` を足す。GitHubでは `GET /repos/{owner}/{repo}/branches/{branch}` を読む。GitLabでは `GET /projects/:id/repository/branches/:branch` を読む。どちらも先頭のコミットを返す。CIは既存の `CommitChecks` で読む。エージェントのトークンも受信箱のトークンも、今の権限で読める。

baseのブランチは、プロジェクトの設定の `base_branch` とする。エージェントが作るPR/MRのbaseは、いつもこの値だからである。

### D3. 緑に戻ったときの取り込みは、Claudeを起動しない短い処理にする

`baseWait` の `watch` でbaseが `green` になったら、次の手順で取り込む（spec「baseが緑に戻ったときの取り込み」）。

1. issueのロック（実行と同じ `locks/`）を取る。取れなければ次の周期に回す。実行の枠（スロット）は使わない。
2. worktreeの `Prepare` と `FetchIssueBranch` で、worktreeをissueのブランチの最新に合わせる。worktreeのheadが `watch` の `sha` でなければ、取り込まずに見張りをやめる。人がpushした後と同じ扱いである。
3. `git merge --no-edit origin/<base>` を、sandboxの外で、エージェントのgitの身元で実行する。衝突したら `git merge --abort` で戻し、`conflicted` に渡す。この直しは回数に数える。
4. 取り込むものが無ければ（baseが進まずに緑へ戻った）、pushせずに `baseWait` を外す。CIの段は終わったものとして扱い、監査ログに残す。PR/MRのCIは再実行しない（Non-Goals）。
5. `Push` のあと、`fix=base`、新しい `sha`、同じ `cr` を持つdoneの投稿を書き、新しい `sha` で `startWatch` する。doneの本文は、取り込んだbaseのコミットを書く短い文にする。

監査ログには、`<ci.base.red>`（待ち始め、1回）、`<ci.base.merged>`、`<ci.base.conflict>`、`<ci.base.noop>` を残す。

代わりの案：回数に数えない直す実行（`cause=base`）として、Claudeに取り込ませる案（grilling.mdのQ6の2つ目の案）。判断の要らない操作に実行の枠と利用量を使ううえ、上限に当たっていると始められない。

### D4. 待ち始めのコメントは一時的なマーカーで書く

マーカーに状態 `note` を足し、`Transient()` に含める。`reason=base` とする。`Transient` に含めれば、エージェントの状態の読み取り（`agent.go`、`ciwatch.go`）と、受信箱の状態の読み取り（`inbox.go`）が無視する。そのため、issueは受信箱で完了のまま出る。マーカーがあるので、ADR-056の同じアカウントでも、人の返信と見分けられる。

1回だけ書くことは、コメントから導く。最新のdoneの投稿より後に、エージェントの `state=note reason=base` があれば、もう書かない。起動し直しても重ならない。

マーカーの `fix` に `base` を足す（`<marker.Base>`）。受信箱の自動マージは `fix` が空でなければ受け付けるので、判定は変えなくてよい。

### D5. 自動マージの「人のpush」の判定に、短い猶予を置く

取り込みでは、issueは `agent:done` のまま、pushしてからdoneを投稿する。その間にheadがマーカーの `sha` と食い違い、受信箱が依頼を打ち切るおそれがある（Context）。

受信箱の自動マージは、headがマーカーの `sha` と違うのを初めて見たとき、その時刻を依頼に覚えて次に回す。2分たっても違えば、今までどおり `ReasonPushed` で打ち切る。一致すれば、覚えた時刻を消す。猶予の2分は、pushからdoneの投稿までにかかる時間（API呼び出しの数回分）に、受信箱の取り直しの間隔の揺れを足した値である。

代わりの案：取り込みの間だけissueを `agent:running` にする案。着手のコメントの無い `running` を、起動し直したときの回復の処理と受信箱の表示が扱えず、扱いを足す範囲が広い。doneを先に投稿してからpushしても、食い違う向きが逆になるだけである。

### D6. マージのダイアログのbaseのCI

`GET /v1/merge-status` の答え（`inboxapi.MergeStatus`）に、`base_branch` と `base_failed`（失敗したチェックの名前とURLの配列）を足す。受信箱のサービスは、D2と同じ `BranchHead` と `CommitChecks` を受信箱のトークンで呼ぶ。読めなかったときは、どちらも空にして、ほかの欄を今までどおり返す。Web表示は、`base_failed` が空でないときだけ1行を出す。文言は部品の `defineMessages` に日本語と英語で書く。

## Risks / Trade-offs

- ［baseとPR/MRで、同じjobのチェックの名前が違う。GitHubのworkflowが `push` と `pull_request` で別のjob名を使うなど］→突き合わせが外れる。今までどおり直す実行が始まるだけで、悪化はしない。V67で、smarternewsとswitchboardのCIで名前が揃うことを確かめる。
- ［baseが長く赤のままで、PR/MRが受信箱に完了のまま並び続ける］→待ち始めのコメントとマージのダイアログで理由が見える。受信箱の一覧の帯は `serialize-inbox-merges` で扱う。
- ［baseが緑へ戻った瞬間、待っていたPR/MRがいっせいに取り込みとpushを行い、CIが一度に走る］→取り込みはClaudeを起動しないので枠は使わない。CIの混雑はフォージ側の待ちに任せる。直列化はBで行う。
- ［取り込みのmergeコミットの作者がエージェントになる］→今の直す実行のコミットと同じ身元であり、新しい扱いは要らない。
- ［D5の猶予の間に、人が本当にpushしていた］→打ち切りが最大2分遅れる。その間にマージされるのは、マーカーの `sha` がheadと一致したときだけなので、人のコミットは入らない。

## Migration Plan

設定、トークンの権限、状態のファイルは変わらない。エージェントと受信箱のサービスを入れ替えるだけでよい。古い受信箱のサービスは `note` のマーカーを知らないが、知らない状態のマーカーを状態の読み取りで使わない作りかは、実装のときにテストで確かめる。戻すときは、前のバイナリに戻す。`fix=base` のdoneは、古いエージェントでも通常のdoneとして読める。

## Open Questions

- 何も取り込まずに緑へ戻ったとき（D3の4）、PR/MRのCIは赤のまま残る。この場合を人にどう知らせるかは、実際に起きる頻度を見てから決める。今は監査ログだけに残す。
