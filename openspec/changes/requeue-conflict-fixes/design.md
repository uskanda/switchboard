# Design

## Context

衝突を直す実行は、`internal/agent/ciwatch.go` の `conflicted` が始める。この巡の直す回数（`fix.attempt`）が `agent.max_fix_attempts` を超えると、`conflictOutcome` で `waiting(reason=conflict)` を書き戻す。直す回数は `fix.go` の `plan.fixes` が、フォージ上の着手のコメント（`reason=fix`）から数える。

スロットを待つ直す実行は、`pending` に入れずに見張り（watch）の中で覚え、順番待ちより先に始める（ADR-054）。上限で止まった実行は、`agent:ready` と `state=queued reason=limit run=` のコメントに戻し、そのコメントを再開の印として `pending` から再開する（ADR-053）。

## Goals / Non-Goals

**Goals:**
- 衝突の直しの上限超えを、ADR-053と同じ形の順番待ちに乗せる。
- 再起動しても、別の端末からでも、同じように回数を数えられるようにする。

**Non-Goals:**
- CIの失敗とpushの拒否の上限超えは変えない。
- 自動マージ（ADR-048）の判定は変えない。再開の着手は `reason=fix` のままなので、依頼は残る。

## Decisions

### D1. 入り直しは、待ちと同じ書き戻しの経路で書く
`conflicted` で上限を超えたとき、入り直した回数（`plan.requeues`）が上限未満なら、`requeueOutcome` を作る。`state=queued reason=conflict` で、`Run` を持つ結果である。見張りの `giveUp` に入れて `writeGiveUp` で書き戻す。`writeOut` は queued の結果でラベルを `agent:ready` にし、通知しない。書き戻せたら見張りを外し、続けて `resume` で順番待ちに入れる。書き戻しが失敗したら、次のポーリングで同じ結果を書き直す（`writeOut` は同じ `run=` のコメントを2度投稿しない）。

代わりに、`pending` へ直接入れて順番待ちのコメントだけを投稿する案もあった。これだと書き戻しのやり直しと、ラベルの付け替えを別に書くことになる。

### D2. 再開の印は `limitResume` を広げて見つける
`limitResume` は、`run=` を持つ `reason=conflict` の順番待ちのコメントも、再開の印として受け付ける。どちらの印かも返す。`decide` は、エージェントが付けた `agent:ready` に衝突の印があれば、`classify` をせずに衝突を直す実行（`fixRun`、cause は `prompt.FixConflict`）の計画を作る。PR/MRの番号は最後の完了のマーカーの `cr=` から、baseはプロジェクトの設定から取る。`cr=` が見つからなければ、実行しない。

人が `agent:ready` を付け直したときは、今までどおり新しい巡として扱い、印は消す。このとき、上限による再開（`Resumed`）としては扱わない。

### D3. 順番待ちから来た直す実行だけは `pending` に入れる
`launch` は、スロットが無いとき、`pl.fix` があれば `errQueued` を返すだけだった（ADR-054）。`pl.fix.requeue` が0でない直す実行は、ほかのissueと同じく `pending` に入れる。印のコメントを順番待ちのコメントとして使い、別のコメントは付けない（ADR-053と同じ）。`resumes` に入るので、再起動時の `clearQueued` は印を消さない。

### D4. 区切りは着手のマーカーの `requeue=` に残す
順番待ちのコメントは再開のときに消えるので、区切りにはならない。順番待ちから再開した直す実行の着手に `requeue=<巡の番号>` を付ける（`marker.Marker.Requeue`）。`fixes()` は、`requeue=` を持つ着手を数えたところか、`reason=conflict` の順番待ちのコメントのところで数えるのをやめる。前者は再開した実行そのものを1回と数え、後者は再開の前の計画で0回と数えるためである。`requeues()` は、信頼済みの人のコメントか `mode` を持つ着手から後の、`requeue=` を持つ着手を数える。

### D5. 順番が来たら衝突を確かめずに始める
直す実行は `git merge` でbaseを取り込む。衝突が無くなっていても害は無い。確かめる問い合わせを足さない。

## Risks / Trade-offs

- [順番待ちのあいだ、受信箱に完了の行とマージのボタンが出ない] → PR/MRは衝突していて、どのみちマージできない。
- [`max_fix_attempts` が大きいと、直せない衝突で使用量を使い続ける] → 入り直しを `max_fix_attempts` 回で打ち切り、`waiting(reason=conflict)` にする。
- [古い版のエージェントが `requeue=` を知らない] → 知らないキーは読み飛ばす。古い版は入り直しをせず、回数も今までどおり数える。
