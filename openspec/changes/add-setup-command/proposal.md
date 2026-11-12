# Proposal

## Why

実行のsandboxはネットワークに出られない（ADR-057）。そのため、依存パッケージを取得しないと動かないプロジェクトでは、実行の中でテストを1件も走らせられない。smarternewsのissueの107番では、worktreeの `node_modules` が空で、`pnpm exec vitest run` がレジストリへ接続できず失敗した。`own-run-sandbox` の設計は、この場合の対策を「必要になったら、プロジェクトごとの設定項目を足す」と書いて先送りしていた。

## What Changes

- プロジェクトの設定に `<projects[].setup_command>`（準備のコマンド）を足す。全プロジェクト共通の既定は作らない
- 準備のコマンドのあるプロジェクトでは、エージェントが実行のたびに準備のコマンドをworktreeで走らせる。走らせるのは、worktreeの準備のあとで `claude -p` の起動の前である。計画の実行、承認のあとの実行、直す実行のすべてが対象である
- 準備のコマンドは、sandboxの外で、`verify_command` と同じ環境で走らせる。フォージのトークンは渡さない。時間の上限は `timeout` を使う
- 準備のコマンドが失敗したときは、`claude -p` を起動せず、実行を `waiting(reason=error)` として扱う。コメントにコマンドと出力の末尾を書く
- 準備のコマンドがworktreeにgitから見える変更を残したときも、準備の失敗として扱う。コメントに変わったパスを書く
- 監査ログに `<setup.ran>` を残す。出力の全体は、実行のディレクトリの `<setup.log>` に残す
- 追記するシステムプロンプトに、依存が準備のコマンドで用意済みであることと、sandboxからは依存を取得できないことを書く
- `claude -p` には準備のコマンドを走らせる許可を与えない。sandboxの外で動くのは、今までどおり `git` と `verify_command` だけである
- ADRを1つ足す。`docs/GUIDE.md` と `docs/SECURITY.md` は、日本語と英語の版を同じ変更で直す

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

- `agent-run`: 準備のコマンドの実行、失敗したときの扱い、記録、`claude -p` への知らせ方の要件を足す
- `config`: `<projects[].setup_command>` の設定と読み直しの要件を足す

## Impact

- `internal/config`（新しい項目）
- `internal/agent/run.go`（準備のコマンドの実行、失敗の書き戻し、監査ログ）
- `internal/agent/messages.go`（失敗のコメントの文言、日本語と英語）
- `internal/prompt`（追記するシステムプロンプトの項）
- `docs/DECISIONS.md`（ADR）と `docs/DESIGN.md`
- `docs/GUIDE.md` と `docs/GUIDE.ja.md`
- `docs/SECURITY.md` と `docs/SECURITY.ja.md`
- 準備のコマンドを設定しないプロジェクトの動きは変わらない
