# Design

## Context

動機はproposal.mdのWhyを参照。今の作りの要点は次のとおり。

- `conflicted` と `ciFailed`（`internal/agent/ciwatch.go`）は、`hasSlot` が偽なら何もせずに戻る。次に試すのは `checkWatches` の次の回で、衝突だけを見る段では `reconcile_interval` ごとになる。
- 空いたスロットは先に取られる。`afterRun` では `drain` が、`poll` では `pollProject` による直接の `launch` と `drain` が、`checkWatches` より前に動く。中継の `refresh` も同じ。
- launchはすべて `pollMu` の下で順に行われる（#25）。

## Goals / Non-Goals

**Goals:**

- スロットが空いた時点で、待っている直す実行が順番待ちより先にスロットを取る。
- 直す実行を始める前に、PR/MRとissueを読み直す。

**Non-Goals:**

- 直す実行を `pending` に入れること、順番待ちのコメントを付けること（第1ラウンドQ3）。
- 設定での切り替え、動いている実行の中断（第1ラウンドQ4）。
- 再起動をまたいで待ちを覚えること。再起動のあとは、見張りの探索と確認で待ちが作り直される。

## Decisions

### D1 待ちはwatchに持たせる

`watch` に `fixWaiting`（`prompt.FixConflict` か `prompt.FixCI`。空なら待っていない）と `fixSince`（待ち始めた時刻）を足す。`conflicted` と `ciFailed` は、スロットが無いときと `launch` が `errQueued` を返したときに記録する。初めて記録したときと、待つ理由が変わったときだけ、1行のログを出す。

待ちを消すのは、状況が確かに変わったときだけにする。衝突が無いと読めたら衝突の待ちを消し、CIが失敗していないと読めたらCIの待ちを消す。直す回数の上限に達して待ちにするときも消す。直す実行を始めたとき、issueがdoneでなくなったときは、`unwatch` でwatchごと消える。フォージの読み取りの一時的な失敗では消さない。消すと待ち始めた時刻が失われ、順が崩れるからである。

別案として、独立した待ちの表を作る形も考えた。しかし、待ちはwatchが続くあいだだけ意味を持つ。watchの項目にすれば、`unwatch` でwatchと一緒に消える。管理も二重にならない。

### D2 `startFixes` で待つ直す実行を先に起動する

待つwatchのうち、実行中でないものを並べる。順は、衝突、CIの失敗、待ち始めた時刻、refの文字列の順とする。端末の上限に達したらそこで止め、プロジェクトの上限に当たったものだけ飛ばす。`drain` と同じ扱いである。

スロットのあるものには `checkWatch` を `force` 付きで呼ぶ。`force` は、衝突だけを見る段の `reconcile_interval` の間引きを飛ばす。`checkWatch` はPR/MRのhead、衝突、CIを読み直し、`fixPlan` でissueを読み直してから `launch` する。状況が変わっていれば、今の処理どおり見張りを外すか待ちを解く。

スロットがあるときだけフォージを読むので、待つあいだに読み取りが増えることはない。

### D3 起動する箇所の先頭で呼ぶ

- `drain` の先頭で呼ぶ。`afterRun`、`poll`、`refresh` の `drain` はみなここを通る。
- `poll` と `refresh` では、`pollProject` の前にも呼ぶ。新しくreadyになったissueが直接の `launch` でスロットを取るより先に、直す実行へ回すためである。

`afterRun` では、終わった実行のissueに返答が来ていれば、`drain` の前に `resume` が同じissueの次の実行を始める。計画どおり、ここは変えない。同じissueのやりとりの続きであり、その実行が使っていたスロットを引き継ぐ形になるからである。

## Risks / Trade-offs

- 待つ直す実行が多いと、スロットが空くたびに1件ずつフォージを読む。→ 読むのはスロットがあるときの先頭の1件だけで、始めれば待ちから外れる。状況が変わっていれば待ちも消える。
- 再起動で、待ち始めた時刻が消える。→ ADR-046の順番待ちも同じ扱いである。再起動のあとの確認で、待ちは作り直される。
- 直す実行が続くと、順番待ちのissueが後回しになり続ける。→ 直す実行は回数の上限（`agent.max_fix_attempts`）があり、PR/MRごとに数が限られる。
