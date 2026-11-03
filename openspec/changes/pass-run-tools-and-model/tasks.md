# Tasks

## 1. 起動の引数

- [x] 1.1 `claudecode.Request` に `Tools`、`Model`、`Effort` を足し、`Args` が `--tools`、`--model`、`--effort` を付けるようにする。空の `Model` と `Effort` ではフラグが付かないことを `claudecode_test.go` で確かめる
- [x] 1.2 effortの許容値の一覧 <claudecode.Efforts> を足し、テストで一覧の値を確かめる

## 2. 設定

- [x] 2.1 `config.Agent` に `model`、`effort`、`plan`、`fix` を足す。項目が無い設定と、計画の実行だけの上書きを、設定のテストで確かめる
- [x] 2.2 effortの値を読み込みで検査する。`ultra` が誤りになり、知らないモデル名が誤りにならないことをテストで確かめる
- [x] 2.3 `config/` の設定の例と、設定を説明する文書に新しい項目を書く

## 3. 実行に渡すツールとモデル

- [x] 3.1 `internal/agent` に、ファイルを書く実行のツールの一覧と、計画の実行のツールの一覧を作り、許可に `WebSearch` を足す。テストで両方の一覧を確かめる
- [x] 3.2 `allow_tools_extra` の各項目からツール名を取り出して足す。`Bash(openspec *)` と `Skill` から `Bash` と `Skill` が取れ、重複しないことをテストで確かめる
- [x] 3.3 `internal/agent` で、実行の種類からモデルとeffortを決めて `Request` に入れる。計画の実行、直す実行、実装の実行の3つをテストで確かめる
- [x] 3.4 `realclaude` のタグの付いたテストを足し、実物の `claude` で、実装の実行と計画の実行のinitの `tools` を確かめる

## 4. 文書

- [x] 4.1 `docs/DECISIONS.md` にADRを足す（ツールの固定のリスト、WebSearchの許可、モデルとeffortの設定）
- [x] 4.2 `docs/ROADMAP.md` の検証項目に、`--tools`、`--model`、`--effort` の確認結果を書く
- [x] 4.3 `docs/DESIGN.md`、`docs/SECURITY.md`、`README.md` の起動と許可の説明を直す

## 5. 確認

- [x] 5.1 `make verify` が通ることを確かめる
- [x] 5.2 `openspec validate pass-run-tools-and-model --strict` が通ることを確かめる
