# Proposal

## Why

実行のsandboxは、worktreeと `$TMPDIR` にしか書き込みを許さない（ADR-057）。Goを使うプロジェクトの実行では、最初の `go build` や `go test` が `read-only file system` で失敗する。既定のビルドキャッシュ（`~/.cache/go-build`）に書けないためである。2026-11-04から11-09までのGoを使う実行のほぼすべてで、Claudeは失敗のあとに `GOCACHE=$TMPDIR/gocache` を前に付けて再実行していた。1ターンを無駄にするうえ、キャッシュが毎回空から作られる。計画の実行では、前置きの `変数=値` が拒否されるので（V58）、この回避の方法も使えない。

## What Changes

- エージェントは、`claude -p` の環境に `GOCACHE` を入れる。値はswitchboard専用のビルドキャッシュの置き場所（既定 `~/.cache/switchboard/go-build`）で、全プロジェクトで1つとする
- エージェントは、実行ごとのsandboxの設定で、この置き場所への書き込みを許す。worktreeと `$TMPDIR` のほかに書けるのは、この置き場所だけとする
- エージェントは、claudeを起動する前に、この置き場所を作る
- 準備のコマンドと検証コマンドも同じ `GOCACHE` で動く。利用者自身の `~/.cache/go-build` には、実行の中からもsandboxの外からも書かない
- sandboxの中での作業の指示に、`GOCACHE` は設定済みで書き換えないことを書く
- sandboxの設定の `filesystem.allowWrite` が働くかを、新しい検証項目V65として実物のclaudeで確かめてから使う
- ADR-057の書き込みの範囲を変える新しいADR（ADR-073）を足し、GUIDEとSECURITYの日本語と英語の版を直す
- モジュールの取得は、今までどおり準備のコマンド（`go mod download` など）で行う
- 準備のコマンドを書くのは利用者の設定の変更で、このchangeではそのためのコードを変えない

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- `agent-run`: `claude -p` の環境変数に `GOCACHE` を加える。sandboxの中から書けるGoのビルドキャッシュの置き場所を定める

## Impact

- `internal/claudecode`：環境変数の組み立てと、`settings.json` のsandboxの `filesystem.allowWrite`
- `internal/agent`：キャッシュの置き場所の決定と作成、実行の設定への受け渡し
- `internal/agent`：準備のコマンドと検証コマンドの環境
- `internal/prompt`：sandboxの中での作業の指示
- `docs/DECISIONS.md`（ADR-073）と `docs/ROADMAP.md`（V65）
- `docs/GUIDE.md`、`docs/GUIDE.ja.md`、`docs/SECURITY.md`、`docs/SECURITY.ja.md`
- 未アーカイブの `own-run-sandbox` の「実行のsandbox」の書き込みの範囲
