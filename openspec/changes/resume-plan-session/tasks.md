# Tasks

## 1. 検証項目

- [x] 1.1 `docs/ROADMAP.md` の検証項目に、再開のときのセッションID、ツール、スキーマ、システムプロンプト、キャッシュの振る舞いと、無いセッションの誤りの形を書く

## 2. claudecodeのアダプタ

- [x] 2.1 `claudecode.Request` に引き継ぐ元のセッションIDを足す。`Args` が `--resume` と `--fork-session` を付けることを、`claudecode_test.go` で確かめる
- [x] 2.2 `claudecode` に、引き継ぐ元のセッションが無い誤りを見分ける関数を足し、記録したresultでテストする

## 3. 設定

- [x] 3.1 `config` に `<agent.resume_within>` を足す。既定の1時間、0、負の値の誤りをテストで確かめる

## 4. 引き継ぐ条件

- [x] 4.1 `<launch.json>` を実行ごとに書く。中身はセッションID、始まりのHEAD、ツール、モデル、effort、追記する指示のSHA-256とする
- [x] 4.2 `<launch.json>` の前回の値と最新の待ちのマーカーから、引き継ぐ元のセッションを決める。端末、理由、間隔、指示、ツール、モデル、effort、HEADの各条件をテストで確かめる
- [x] 4.3 `approved` と `fix` の実行では引き継がないことを、テストで確かめる

## 5. 引き継いだ実行

- [x] 5.1 `prompt.Input` に前の段の続きを示す項目を足し、`Build` が先頭に行を書くことをテストで確かめる
- [x] 5.2 `No conversation found` で引き継ぎに失敗したとき、同じ実行の中で、新しいセッションIDを使って起動し直す。偽のclaudeでテストする。結果のコメントは1件で、マーカーは起動し直したあとのセッションを持つ
- [x] 5.3 `run.started` に `resumed_from` を足し、起動し直したときの2行目をテストで確かめる

## 6. 実物での確認

- [x] 6.1 `realclaude` のタグの付いたテストを足す。計画の実行を2回続け、2回目のinitとresultのセッションIDが新しいIDで、前回の内容を覚えていることを確かめる

## 7. 文書

- [x] 7.1 `docs/DECISIONS.md` にADRを足す（引き継ぐ実行の種類、条件、失敗したときの起動し直し、用語）
- [x] 7.2 `README.md` に `<agent.resume_within>` と引き継ぎの説明を書き、`docs/DESIGN.md` の4.4節の起動の説明を直す

## 8. 確認

- [x] 8.1 `make verify` が通ることを確かめる
- [x] 8.2 `openspec validate resume-plan-session --strict` が通ることを確かめる
- [ ] 8.3 `run.started` の `resumed_from` を使い、リリースのあとの実ログで、引き継いだ計画の実行のキャッシュの読み込みとターン数を、引き継がない実行と比べる
