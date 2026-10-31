# Tasks

## 1. 実装

- [x] 1.1 `config.Relay` に、プロジェクトが `relay.hooks` に載っているかを引く関数を足す。refの大文字と小文字を区別しないことを、テストで確かめる
- [x] 1.2 エージェントのループを直す。中継につながっている間も、`relay.hooks` に載っていないプロジェクトは `poll_interval` ごとにポーリングする。載っているプロジェクトは `<reconcile_interval>` ごとに照合する。全プロジェクトが載っているときだけ、ループの間隔を `<reconcile_interval>` にする。偽の中継とフォージで、失敗するテストから確かめる

## 2. 文書と確認

- [x] 2.1 README.mdの中継の節で、照合の間隔を使うのは `relay.hooks` に載ったプロジェクトだけだと書き直す
- [ ] 2.2 usknasに新しい版を置き、中継につながった状態で、`gh:uskanda/switchboard` のissueが `poll_interval` のうちに評価されることを確かめる。結果はROADMAP.mdに記録する
