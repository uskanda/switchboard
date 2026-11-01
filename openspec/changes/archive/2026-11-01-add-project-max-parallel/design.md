# Design

## Context

動機はproposal.mdの「Why」にある。現状のコードは次のとおり。

- `internal/agent/agent.go` の `launch` は、`len(a.active)` を `a.Config.Agent.MaxParallel` と比べる。枠が無いと、issueを `pending` に積んで `errQueued` を返す。
- `drain` は、名前順に並べた `pending` を先頭から見る。全体の枠が埋まると、そこで取り出しを止める。
- `pending` は、積んだときの `config.Project` を値で持つ。読み直しの後も古いプロジェクトの設定が残る。
- `internal/config/watch.go` の `Watcher` は、`projects` だけを `apply` に渡す。ほかの節の変化は「再起動が要る」とログに残す。`switchboard agent run` と `switchboard inbox serve` の両方がこれを使う。

## Goals / Non-Goals

**Goals:**

- 全体とプロジェクトの2つの上限を、`launch` と `drain` の同じ判定で扱う
- どちらの上限も、再起動せずに読み直しで反映する

**Non-Goals:**

- 順番待ちをissueや受信箱に示すこと（uskanda/switchboard#46で扱う）
- 順番待ちの順番を先着順にすること
- `<agent.max_parallel>` 以外の `agent` の項目を読み直しで反映すること

## Decisions

### 上限の判定を1つの関数にまとめる

`launch` と `drain` は、どちらも「このissueのプロジェクトに、いま枠があるか」を確かめる。`a.mu` を持った状態で呼ぶ判定を1つ作り、両方から使う。判定は次の2つを満たすときに真とする。

- `len(a.active)` が全体の上限より小さい
- プロジェクトに上限があるとき、そのプロジェクトの `active` の数が、プロジェクトの上限より小さい

プロジェクトの `active` の数は、`active` のキーの `IssueRef` から、フォージ・owner・リポジトリ名が一致するものを数えて求める。個人の規模では `active` は数件なので、別の集計を持たない。

### プロジェクトの上限は、判定のたびに今の設定から引く

`pending` が持つ `config.Project` は、積んだときの値である。読み直しでプロジェクトの上限を変えても、`pending` の値は変わらない。そのため、判定ではissueのプロジェクトを今の `a.projects` から引き、その `MaxParallel` を使う。見つからないプロジェクトは、読み直しで消えたものであり、`dropPending` が順番待ちから外す。

### drainは、上限に達したプロジェクトのissueを飛ばす

`drain` は、全体の枠が埋まったときだけ取り出しを止める。プロジェクトの枠が埋まったissueは `pending` に残して、次のissueを見る。

### 全体の上限は、`a.mu` で守る値として持つ

全体の上限は `a.Config.Agent.MaxParallel` から読まず、`a.mu` で守るフィールドに持つ。起動時は設定の値を入れ、読み直しのたびに入れ替える。`a.Config` は複数のゴルーチンから読まれるため、その中身を書き換えない。

### Watcherは、変化をまとめて渡す

`Watcher.Run` の `apply` の引数を、`[]Project` から `*Change` に変える。`Change` に全体の上限を足す。`changedSections` は、`agent` の節を比べるとき `MaxParallel` を除いて比べる。こうすると、`<agent.max_parallel>` だけを変えたときに「再起動が要る」とログに残らない。

`switchboard inbox serve` は `Change` の `Projects` だけを使い、全体の上限は無視する。受信箱は実行を起動しないためである。

別の案として、`Watcher` に専用のコールバックを足す方法を考えた。呼び出し側が2つの関数を渡すことになり、`projects` と上限が別々の時点で反映される。1つの `Change` で渡せば、エージェントは両方を同じ時点で受け取れる。

### 読み直しの後に順番待ちを取り出す

`Reload` は、新しい上限を `projects` と一緒に受け取り、ポーリングのループを起こす。ポーリングは、読み直しを適用してから `poll` を呼ぶ。`poll` は最後に `drain` を呼ぶため、上がった上限で順番待ちのissueが始まる。`drain` を別に呼ぶ処理は足さない。

### ロックの順番

`a.mu` を持ったまま `a.pmu` を取らない。`applyDesired` は `a.pmu` を持ったまま `a.mu` を取るため、逆の順番で取るとデッドロックする。プロジェクトの上限は、`a.mu` を取る前に `a.projectList()` から引く。
### 同じcloneへのgitの書き込みを1つずつにする

実装の途中で、既存の不具合が見つかった。同じプロジェクトの2つの実行が同時に `git worktree add -b` を呼ぶと、gitは `.git/config` のロックで2つ目を拒む。`<agent.max_parallel>` が1の間は、同じcloneで2つの実行が重ならないため表に出なかった。

`workspace` に、cloneのパスごとのミューテックスを置く。`Prepare`（fetchとworktreeの作成）と `Remove`（worktreeとブランチの削除）は、これを持ってからcloneに書き込む。worktreeの中でのcommitとpushは、cloneの `config` を書かないため対象にしない。別の端末やプロセスとの競合は、issueのロックと担当の端末の設定で起きない。

## Risks / Trade-offs

- ［全体の上限を下げても、動いている実行の数はすぐには減らない］→実行が終わるたびに、動いている実行の数は新しい上限へ近づく。`launch` は `len(a.active)` が上限以上なら積むだけなので、上限を超えて新しい実行は始まらない
- ［プロジェクトの上限が全体より大きい設定は、書いた値どおりに動かない］→誤りにはしない（grilling.md q5）。READMEの設定例に、実際の上限は2つの値の小さい方になることを書く

## Migration Plan

既存の設定は変更なしで読める。`<projects[].max_parallel>` が無いプロジェクトは、全体の上限だけで制限され、今までと同じに動く。戻すときは、`<projects[].max_parallel>` を消してから前の版に戻す。前の版は未知のキーを誤りにするためである。
