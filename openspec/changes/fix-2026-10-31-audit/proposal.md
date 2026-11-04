# Proposal

## Why

2026-10-31の監査（HEAD f7272ad）で、外部コマンドが止まらない、issueが二度と実行されない、信頼の判定をすり抜けられる、Web表示が全インターフェースで待ち受ける、といった不具合が見つかった。文書もフックを前提に書かれていて、実装と食い違う。issue #28は、これらを現在のmainに照らして確かめ直し、まとめて直すよう求めている。

## What Changes

- `verify_command` と `check_command` を、時間切れと取り消しでプロセスグループごと止める
- ADR-018の検査を、最新のベースではなくissueのブランチの分岐点と比べる。下の階層の `CLAUDE.md`、`AGENTS.md`、`.claude/` も検査し、保護パスからのリネームも拒む
- GitHubのコメントとGitLabのノートを全ページ読み、編集者を確かめる。issueのタイトルの編集者も確かめる。GitLabの本文の編集者は「changed the description」のシステムノートで決める
- Web表示の `listen` を、ループバック、プライベート、リンクローカルのアドレスに限る。すべての要求で `Host` を確かめる
- 複数の端末の競合、回復処理、起動時の `RepoPrivate` の失敗、中継、サービスのunit、`CredentialName` の衝突、受信箱の検索範囲、`state.WriteJSON` の一時ファイルの不具合を直す
- 設定 `agent.retention` を足し、古い実行ディレクトリを消す
- `doctor` がトークンの権限を診断する。プロンプトが、GitLabのissueを「GitHub issue」と書かない
- 文書を実装に合わせる。公開リポジトリを対象にするADRと、フックを使わないADRを足す
- テストを足し、`make verify` で `go test -race` を実行する
- 権限規則の文法、検索の範囲、投稿の書式、中継のイベントを、それぞれの持ち主のパッケージへ移す（振る舞いは変えない）
- Goを1.26系（`go 1.26.0`、`toolchain go1.26.10`）に上げる

## Capabilities

### New Capabilities

なし

### Modified Capabilities

- `agent-run`: 外部コマンドの停止、ADR-018の検査の比べ先と対象、保護パスのリネーム、回復処理と着手の判定、実行ディレクトリの保持
- `trust`: コメントとノートの全ページの読み取り、タイトルの編集者の検査
- `web`: `listen` に許すアドレスと、すべての要求の `Host` の検査
- `relay`: 空のシークレットの拒否、配信IDの無いwebhookのログ、再起動をまたぐSSEのID
- `config`: `agent.retention` と、`token_ref` のクレデンシャル名の衝突の拒否

## Impact

- コード：`internal/agent`、`internal/workspace`、`internal/forge/github`、`internal/forge/gitlab`、`internal/forge/fake`、`internal/trust`、`internal/relay`、`internal/relayclient`、`internal/service`、`internal/secret`、`internal/state`、`internal/inbox`、`internal/inboxserve`、`internal/doctor`、`internal/prompt`、`internal/claudecode`、`web/internal/webconfig`、`web/internal/server`、`web/cmd/switchboard-web`、`cmd/switchboard`。新しいパッケージ `internal/event` と `internal/postfmt`
- インターフェース：`forge.Forge.SearchOpenIssuesWithLabel` は `forge.Scope` を受け取る
- 設定：`agent.retention`（既定720h）
- 文書：AGENTS.md、`docs/ARCHITECTURE.md`、`docs/DESIGN.md`、`docs/DECISIONS.md`、SECURITY.md、ROADMAP
- ツールチェーン：Go 1.26.10。CIは `go-version-file` で追随する
