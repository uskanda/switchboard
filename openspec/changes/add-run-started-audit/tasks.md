# Tasks

## 1. 指示の読み込みとハッシュ

- [x] 1.1 `Store.Load` を足し、`Plan`/`Run` をその `Text` に書き換え、ハッシュの計算を `Save` と共通にする。`instructions_test.go` で2点を確かめる。`Load` のハッシュが `instructions.saved` の `sha256` と一致すること。ファイルが無いか空白だけのとき、ハッシュが `""` になること

## 2. 実行の開始の記録

- [x] 2.1 `settings()` から足す許可の計算を `extraAllow` に切り出し、既存の `settings()` のテストが通ることを確かめる
- [x] 2.2 `work` で `run.started` を残す。直接の実装、計画、承認後、pushを拒まれたあとのfixの各テストと、指示の中身がログに出ないことのテストで確かめる
- [x] 2.3 `README.md` の追加の指示の節に `run.started` の記録を書き足す

## 3. 全体の確認

- [x] 3.1 `make verify` と `openspec validate add-run-started-audit --strict` が通ることを確かめる
