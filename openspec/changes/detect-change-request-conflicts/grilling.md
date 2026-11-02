# grilling 記録: detect-change-request-conflicts

`agent:done` のissueのPR/MRがbaseと衝突したら、エージェントがそれを見つけ、`git merge origin/<base>` で解消する直す実行を自動で始めるようにする計画である。今の直す実行のきっかけ（ADR-041）は、CIの失敗とpushのnon-fast-forwardだけである。衝突したPRでは `pull_request` のCIが走らないので、CIの見張りは「no CI ran on the commit」で黙って終わる。PR #71（issue #67）では、doneの後にmainへ別のPRが入って衝突し、そのまま止まっていた。

## 決定

| 決定 | 選択 | 出典 |
|---|---|---|
| 衝突を見張る期間 | issueがopenで `agent:done`、かつPR/MRがopenの間はずっと見張る。問い合わせは `reconcile_interval` ごとに、doneのissue1件につき変更要求を1回読む。pushから6時間というCIの見張りの期限は衝突には使わない。baseのheadの変化をきっかけにする案は、仕組みが増えるので採らない | 第1ラウンド Q1 |
| 直す対象のhead | 変更要求のheadが、エージェントの最新の完了のコメントのマーカーの `sha` のままのときだけ自動で直す。人がpushした後の衝突は人に任せる（ADR-041と同じ線） | 第1ラウンド Q2 |
| 直す回数 | CIの失敗、non-fast-forwardと合わせて `agent.max_fix_attempts` を共有する。0なら衝突も自動では直さない。数え直しの規則も同じ | 第1ラウンド Q3 |
| 上限に達したとき | 新しい `waiting(reason=conflict)` にし、受信箱でCIの失敗と見分けられるようにする。reasonの表示用の言葉も足す。直す実行の中でClaudeが両方の変更の矛盾を解消できないと判断したときは、non-fast-forwardの直しと同じく `waiting` で質問させる | 第1ラウンド Q4 |
| 衝突とみなす状態 | GitHubは `mergeable=false` かつ `mergeable_state=dirty`、GitLabは `has_conflicts=true` だけを衝突とする。`mergeable=null` などの計算中は次の問い合わせに回す。`behind`（衝突なしでbaseより遅れている）や `blocked` は対象外 | 第1ラウンド Q5 |
| CIの失敗と衝突が同時にあるとき | 衝突を先に直す。mergeのpushでCIが走り直すので、CIの失敗はその結果で改めて判定する | 第2ラウンド Q6 |
| 対象のフォージ | GitHubとGitLabを同じ変更で入れる。どちらも今のトークンの権限で読める | 第2ラウンド Q7 |
| 記録の残し方 | ADR-043を足し、ADR-041のきっかけにbaseとの衝突を加えることと、見張りを変更要求がopenの間に延ばすことを残す。SECURITY.mdの4章も書き換え、ci-watchのspecに要求を足す。ADR-041そのものは書き換えない | 第2ラウンド Q8 |

## 後回しにしたもの

- baseのheadの変化をきっかけに問い合わせを減らすこと。doneのissueが増えて問い合わせが問題になってから扱う
- `behind` のときにbaseを取り込むこと
- 人がpushした後の衝突を自動で直すこと

## 状態

frontierは空。共有理解は2026-11-02に確認済み。
