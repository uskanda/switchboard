# Tasks

## 1. 待つ直す実行を先に起動する

- [x] 1.1 `queue_order_test.go`に、衝突を直す実行が順番待ちより先に始まり、queuedのマーカーが付かないことを確かめる失敗するテストを書く
- [x] 1.2 CIの失敗を直す実行が、prefer_planがオンでも計画の実行より先に始まるテストを書く
- [x] 1.3 `conflict_test.go`に、衝突だけを見る段で、`reconcile_interval`の前でもスロットが空けば直す実行が始まるテストを書く
- [x] 1.4 `conflict_test.go`に、衝突がCIの失敗より先に始まるテストと、待つあいだに衝突が消えると始まらず、待ちのログが1回だけ出るテストを書く
- [x] 1.5 `queue_order_test.go`に、プロジェクトの上限に当たった直す実行が、ほかのプロジェクトの順番待ちを止めないテストを書く
- [x] 1.6 pollで新しくreadyになったissueが、待っている直す実行より先にスロットを取らないテストを書く
- [x] 1.7 `watch`に`fixWaiting`と`fixSince`を足し、`conflicted`と`ciFailed`で記録する。1.1〜1.6のテストが通ることを確かめる
- [x] 1.8 `startFixes`を足し、`drain`の先頭と、`poll`と`refresh`の`pollProject`の前で呼ぶ。1.1〜1.6のテストが通ることを確かめる
- [x] 1.9 `launch`、`conflicted`、`ciFailed`、`queueOrder`、`drain`の説明を新しい動きに合わせる。`go test ./internal/agent/...`が通ることを確かめる

## 2. 文書

- [x] 2.1 `docs/DECISIONS.md`にADR-054を足し、textlintが通ることを確かめる
- [x] 2.2 `docs/DESIGN.md`のschedulerの節と、`README.md`の同時実行の節に、直す実行が順番待ちより先に始まることを足す。textlintが通ることを確かめる

## 3. 全体の確認

- [x] 3.1 `make verify`と`openspec validate prefer-fix-runs-in-queue --strict`が通ることを確かめる
