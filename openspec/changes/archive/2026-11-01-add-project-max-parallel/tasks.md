# Tasks

## 1. 設定

- [x] 1.1 `config.Project` に `MaxParallel` を足し、1より小さい値を誤りにする。0の誤りと、全体より大きい値を受け付けることを `internal/config` のテストで確かめる
- [x] 1.2 `Change` に全体の上限を足し、`changedSections` が `agent` の `MaxParallel` の変化を無視するようにする。`<agent.max_parallel>` だけを変えたファイルで `Ignored` が空になり、`<agent.poll_interval>` を変えると `agent` が残ることをテストで確かめる
- [x] 1.3 `Watcher.Run` の `apply` を `*Change` を受け取る形に変え、`cmd/switchboard` の2つの呼び出し側を直す。`go build ./...` と既存の `Watcher` のテストが通ることで確かめる

## 2. エージェント

- [x] 2.1 `a.mu` で守るフィールドに全体の上限を移し、`launch` と `drain` が使う枠の判定を1つにまとめる。プロジェクトの上限に達したとき2つ目が順番待ちになることを `internal/agent` のテストで確かめる
- [x] 2.2 `drain` が、プロジェクトの上限に達したissueを飛ばして次のissueを実行するようにする。spec「前のissueのプロジェクトが上限に達している」の筋書きをテストで確かめる
- [x] 2.3 `Reload` が全体の上限を受け取り、適用したあとのポーリングで `drain` が新しい上限を使うようにする。上限を1から2に上げると順番待ちが始まること、2から1に下げても動いている実行が止まらないことをテストで確かめる

- [x] 2.4 `git worktree add` を同じcloneで同時に呼ぶと、gitが `.git/config` のロックで2つ目を拒む。`Prepare` と `Remove` のcloneへの書き込みを、cloneごとに1つずつにする。6つの `Prepare` を同時に呼んで全部が成功することを `internal/workspace` のテストで確かめる

## 3. 文書

- [x] 3.1 `README.md` と `docs/DESIGN.md` の設定例に `<projects[].max_parallel>` を足し、実際の上限は2つの値の小さい方になることを書く。どちらも読み直しで反映されることも書き、文書をtextlintで確かめる

## 4. 確認

- [x] 4.1 `make verify` が通ることを確かめる
