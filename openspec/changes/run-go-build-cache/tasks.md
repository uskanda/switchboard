# Tasks

## 1. allowWriteの確認（V65）

- [x] 1.1 `internal/agent/realclaude_test.go` に、`filesystem.allowWrite` に書いた置き場所へsandboxの中から書けることを確かめるテストを足す。worktreeと `$TMPDIR` には書けて、ほかの場所（利用者の `~/.cache/go-build` に当たる場所）には書けないことも確かめる。`go test -tags realclaude -run RealClaudeSandbox ./internal/agent/` を実物のclaudeで動かし、通ることを確かめる
- [x] 1.2 `docs/ROADMAP.md` のV65に、結果とclaudeの版と日付と再現の手順を書く。結果が設計と違うときは、ここで止めてユーザーに戻す

## 2. sandboxの設定と環境

- [x] 2.1 `claudecode.Settings` に書き込みを許すパスを足し、`settings.json` の `sandbox.filesystem.allowWrite` に書く。空のときも空の配列を書く。`internal/claudecode` の単体テストで、書いたファイルの中身を確かめる（先に失敗するテストを書く）
- [x] 2.2 `internal/claudecode` の環境の組み立てに、決めた `GOCACHE` を加える形を作る。親の環境の `GOCACHE` は渡さない。単体テストで、親の環境に `GOCACHE` があるときとないときを確かめる（先に失敗するテストを書く）
- [x] 2.3 `internal/agent` で置き場所を求め、claudeを起動する前に作る。置き場所は、`$XDG_CACHE_HOME`（無ければ `$HOME/.cache`）の下の `switchboard/go-build` である。実装の実行と計画の実行の両方の設定と環境に渡す。偽のclaudeを使う既存のテストで確かめる。確かめるのは、`settings.json` の `allowWrite`、claudeの環境の `GOCACHE`、置き場所の作成である
- [x] 2.4 `GOCACHE` を、準備のコマンドと、エージェントが動かす検証コマンドの環境にも入れる。既存の準備のコマンドのテストで、環境の `GOCACHE` を確かめる

## 3. 指示

- [x] 3.1 `internal/prompt` のsandboxの中での作業の指示に、`GOCACHE` は設定済みで書き換えないことを1行足す。計画の実行の指示にも同じ行が入るようにする。`internal/prompt` のテストで確かめる

## 4. 文書

- [x] 4.1 `docs/DECISIONS.md` にADR-073を足す。ADR-057の書き込みの範囲に、Goのビルドキャッシュの置き場所を加えることと、その理由（実行の数と回避の方法、V58、V65）を書く
- [x] 4.2 `docs/GUIDE.ja.md` と `docs/GUIDE.md` の「実行のsandbox」に、書ける場所としてGoのビルドキャッシュの置き場所を足す。モジュールは準備のコマンド（`go mod download`）で取得することも書く
- [x] 4.3 `docs/SECURITY.ja.md` と `docs/SECURITY.md` のT7に、書ける場所の追加と、キャッシュを通して新しく得る力が無い理由を書く
- [x] 4.4 `own-run-sandbox`（未アーカイブ）の「実行のsandbox」の要件の書き込みの範囲に、Goのビルドキャッシュの置き場所を加える。`openspec validate own-run-sandbox --strict` が通ることを確かめる

## 5. 結合の確認

- [x] 5.1 `make verify` を通す
- [ ] 5.2 `stream.jsonl` で、エージェントを入れ替えたあとのGoのプロジェクトの実行の最初の `go test` が、`read-only file system` で失敗しないことを確かめる
