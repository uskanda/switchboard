# Design

## Context

動機はproposal.mdの「Why」を参照。今のコード（main、d05f465）で、この設計を縛る点は次のとおり。

- `evaluateOnce` は、始めるラベルと `agent:waiting` のissueだけを `consider` に回す。`decide` も、その2つのほかは見送る
- `hasReply` は、エージェントの最新の投稿（着手と進捗と順番待ちを除く）のマーカーが `waiting` のときだけ、その `ack` より後の信頼済みのコメントを返答とみなす
- `finish` は、doneのたびに `CreateChangeRequest` を呼ぶ。同じブランチの2回目は、GitHubが422、GitLabが409を返す
- worktreeを用意する `Prepare` は、実行のたびにoriginのbaseをfetchする。worktreeは前の実行のものを使い続ける。pushはforceなしの `git push origin b:b` である
- 受信箱の `list` は、待ち、実行中、始めるラベルのラベルで検索する。行番号を振るのは待ちの行だけで、`List` はその並びを行番号のキャッシュに残す
- `inbox.Item` と `inboxapi.Item` は、型の変換でつながっている。項目は両方で同じにする

## Goals / Non-Goals

**Goals:**

- doneからの再開を、waitingからの再開と同じ経路（`consider`、`claim`、`launch`）で行う
- PR/MRの扱いを、フォージの操作1つで決める

**Non-Goals:**

- force push。人間がPRのブランチに積んだコミットを消すおそれがある
- PR/MRの本文の書き直し。最初の本文のままにする

## Decisions

### D1. doneを再開の対象に入れる

`evaluateOnce` と `decide` で、`agent:done` を `agent:waiting` と同じ枝に入れる。`hasReply` は、最新の投稿のマーカーが `done` のときも返答を探す。人間が手で `agent:done` を付けただけのissueには、エージェントのdoneの投稿が無いので、再開しない。

実行の終わり（`afterRun`）と書き戻しの再試行のあとの `resume` も、doneで終わったときに呼ぶ。実行の途中に届いたコメントは、doneの投稿の `ack` より後にあるので、そのまま次の実行になる（ADR-016）。

GitLabでは、`pollReplies` と `keepNotes` の検索に `agent:done` を足す。コメントでissueの更新時刻が変わらないため（V22）である。

### D2. 開いたPR/MRを探す

`forge.Forge` に `FindOpenChangeRequest(ctx, owner, repo, head) (url, found, err)` を足す。

- GitHub：`GET /repos/{o}/{r}/pulls?head={o}:{branch}&state=open`
- GitLab：`GET /projects/:id/merge_requests?source_branch={branch}&state=opened`

`finish` はpushの後にこれを呼び、見つかればそのURLでdoneにする。投稿の行は「PRを更新しました：URL」（MRなら「MRを更新しました」）にする。探す操作の失敗は、PR/MRを作れなかったときと同じ文言で、waiting（error）にする。

代わりに、作成の失敗（422、409）の本文から既存のURLを読む案もあった。誤りの本文の形に頼るので採らない。

### D3. 衝突はmergeで解消させる

`prompt.System` と `prompt.ApprovedSystem` にbaseのブランチ名を渡す。作業の規則に、次の3点を足す。

- baseと衝突したら `git merge origin/<base>` で解消し、mergeをcommitする
- rebase、amend、履歴の書き換えはしない
- switchboardはforceなしでpushする

実装する実行の許可ツールに `Bash(git merge)` と `Bash(git merge *)` を足す。計画の実行の許可ツールは変えない。

### D4. 受信箱の状態 `done`

`inbox.Item` に状態 `done` と項目 `DoneSince`、`ChangeURL` を足す。`DoneSince` と端末は、エージェントの最新のdoneの投稿のマーカーから取る。`ChangeURL` は、その投稿の見出しの次の段落にある最初のURLである。

並びの順位は、待ち、done、実行中、順番待ちとする。行番号を振る状態が先に並ぶので、番号は待ちからdoneへ途切れずに続き、行番号のキャッシュの仕組みは変えずに済む。表示（`Print`、WUI）は節ごとに並べ、doneの節を順番待ちの下に置く。

`Detail` に `Done` を足す。`Reply` は、openで `agent:waiting` か `agent:done` のissueに投稿する。

### D5. 計画を先に立てるモード

`classify` は変えない。最新の `mode=plan` の着手の後に計画と承認があれば、doneの後のコメントで再開してもapprovedRunになる。

## Risks / Trade-offs

- ［doneのissueに付いた「ありがとう」のような信頼済みのコメントでも実行が始まる］→closeすれば止まる。閉じる前にコメントするときは、作業を頼む内容にする。READMEに書く
- ［人間がPRのブランチに直接pushしていると、switchboardのpushが拒まれる］→今までどおりwaiting（error）として受信箱に出る
- ［doneのissueが溜まると、受信箱の一覧が長くなる］→ヘッダの件数で気付ける。closeすれば消える
