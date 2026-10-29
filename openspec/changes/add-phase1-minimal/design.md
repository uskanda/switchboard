# Design

## Context

動機と範囲はproposal.mdにある。前提は次の3つである。

- 判断はgrilling（`grilling.md`）とDECISIONS.mdのADRで決めた。特にADR-010（Go）、ADR-011・012（書き込みとpushはエージェント）、ADR-021（ハーネス非依存）、ADR-022（doneの条件）、ADR-027（認証方式）に従う
- Claude Codeの挙動は、Phase 0でROADMAP.mdの検証項目に記録した。確認した版は2.1.312である
- 最初の実装はUbuntuの1台を対象とし、GitHubの1リポジトリをポーリングで扱う

## Goals / Non-Goals

**Goals:**

- specsの6つのcapabilityを、外部仕様への依存を薄い層に閉じ込めて実装する
- Claude Codeとフォージを偽物に差し替えて、遷移と判定をテストで網羅する
- ハーネスを入れないCIで、検証が通る

**Non-Goals:**

- GitLab、中継、常駐化（systemd）、別アカウントでの投稿、専用OSユーザー：Phase 2以降で扱う
- `--resume` による再開（grilling第6ラウンドQ5）
- macOSのキーチェーン：Phase 3で扱う

## Decisions

### 1. パッケージの構成

```
cmd/switchboard/            サブコマンドの組み立てだけ
internal/config/            設定の読み込みと検証
internal/secret/            Secret Serviceと、開発時の環境変数
internal/forge/             フォージのインターフェースと型
internal/forge/github/      GitHubの実装（REST + GraphQL 1本）
internal/forge/fake/        テスト用の偽物
internal/labels/            状態ラベルの作成、付け替え、正規化
internal/marker/            マーカーの生成と解析
internal/trust/             信頼判定と監査ログ
internal/prompt/            プロンプトと追記の指示の組み立て
internal/claudecode/        claude CLIの引数、環境、設定ファイル、出力の解析
internal/workspace/         worktree、設定の検査、出力検査、push
internal/agent/             検知、評価、実行、書き戻しのループ
internal/inbox/             一覧、詳細、返答
internal/state/             状態ディレクトリ（ロック、記録、監査ログ）
```

依存の向きは `cmd` → `agent` / `inbox` → `trust`・`labels`・`marker`・`prompt` → `forge`（インターフェース）とする。`forge/github` と `claudecode` は外側に置き、内側からはimportしない。CLAUDE.md（現AGENTS.md）の「外部仕様は薄い層に隔離する」をこの2つで満たす。

### 2. GitHubへの接続

REST APIには `google/go-github` を使う。編集者（`editor`、`lastEditedAt`）はRESTに無いため、issueとコメントの編集者を取るGraphQLのクエリを1本だけ、標準ライブラリのHTTPで送る。

- 代替：GraphQLのクライアントライブラリを使う案は、クエリが1本だけなので依存を増やす利点が小さく、採らない
- ポーリングは `GET /repos/{owner}/{repo}/issues?state=all&since=<前回の取得時刻 - 60秒>` に、ETagの条件付きリクエストを付ける。60秒を重ねるのは、時計のずれによる取りこぼしを防ぐため
- ラベルの付与者は、issue eventsの `labeled` イベントの `actor` から取る

### 3. トークンの保管

Secret Serviceには `zalando/go-keyring` で接続する。項目は、属性 `service` に `token_ref`、属性 `username` に `token` を持つものとする。利用者は次のコマンドで登録できる。

```
secret-tool store --label='switchboard github' service switchboard/github username token
```

- 代替：libsecretをcgoで呼ぶ案は、クロスコンパイルを難しくするため採らない
- ヘッドレスの端末（D-Busのセッションが無い）では使えない。Phase 1では開発時の環境変数で補い、systemdの暗号化クレデンシャルはPhase 3で扱う

### 4. 状態ディレクトリ

`$XDG_STATE_HOME/switchboard`（既定は `~/.local/state/switchboard`）に、次のものを置く。

- `<identities.json>`：ログイン名とユーザーIDの対応
- `<audit.jsonl>`：信頼判定と書き戻しの記録。1行に1件
- `runs/<run-id>/`：プロンプト、追記の指示、Claude Codeの設定、stream-jsonの出力、結果
- `locks/<owner>-<repo>-<n>.lock`：issueごとのファイルロック（`flock`）
- `<poll.json>`：プロジェクトごとの前回の取得時刻とETag

受信箱の行番号の記録は、`$XDG_CACHE_HOME/switchboard/inbox.json` に置く。

- 代替：DESIGN.md 8.2節のSQLiteは、Phase 1の記録の量では利点が小さいため、Phase 1では使わない。件数が増えた時点で見直す

### 5. claude CLIの起動

起動の形は次のとおりとする。

```
claude -p --session-id <uuid> --output-format stream-json --verbose
  --permission-mode dontAsk --settings <run-dir>/settings.json
  --strict-mcp-config --mcp-config <run-dir>/mcp.json
  --append-system-prompt-file <run-dir>/system.md
  --max-turns <max_turns> --json-schema <schema>  < <run-dir>/prompt.md
```

- `--setting-sources` は付けず、既定どおりユーザー層とプロジェクトの設定を読み込む（grilling第6ラウンドQ1）。プロジェクトの `CLAUDE.md` と `AGENTS.md` も、この形で読み込まれる
- `settings.json` の許可は、ファイルの読み書き（Read、Edit、Write、Glob、Grep）と、`git` の `status`・`diff`・`log`・`add`・`commit` に限る。`verify_command` があれば、その先頭のコマンドも許可する。denyには `git push`、`gh`、WebFetchを入れる。Phase 0のV5のとおりdenyは境界にならないため、本体の対策はトークンを渡さないことである
- MCPのサーバーは、空の `mcp.json` と `--strict-mcp-config` で使わせない。プロジェクトの `.mcp.json` のサーバーは、サンドボックスの外で動くためである
- 終了の宣言のスキーマは次のとおりとする

```json
{"type":"object","additionalProperties":false,"required":["status","summary"],
 "properties":{"status":{"type":"string","enum":["waiting","done"]},
  "summary":{"type":"string"},"question":{"type":"string"}}}
```

- プロセスは新しいプロセスグループで起動する。止めるときは、グループにSIGINTを送り、30秒後にSIGTERM、さらに10秒後にSIGKILLを送る。Phase 0の記録（C23）のとおり、SIGTERMから始めるとresultが残らない
- `doctor` は `claude --version` を記録し、確認済みの版より古いときは警告する

### 6. 設定の検査と出力検査

- 実行前の検査は、`git diff --quiet <base_branch> -- .claude .mcp.json CLAUDE.md AGENTS.md` と、同じパスの未追跡ファイルの有無で行う
- 秘密情報の検査は、正規表現で行う。対象は `base_branch` から `HEAD` までの差分、コミットメッセージ、投稿する文章とする。形式は、GitHubとGitLabのトークンの接頭辞、AWSのアクセスキー、秘密鍵のヘッダとする
- pushは、`git push origin switchboard/issue-<n>:refs/heads/switchboard/issue-<n>` の形に固定する。トークンは `GIT_ASKPASS` で、エージェントの子プロセスにだけ渡す

### 7. テストの方針

- フォージは `forge/fake` に差し替え、遷移表（specsの `agent-run` と `run-outcome`）をシナリオごとにテストする
- claude CLIは、Phase 0で記録したstream-jsonを返す偽の `claude` スクリプトを `PATH` の先頭に置いてテストする
- 結合テストでは、実物のgitリポジトリ、偽のフォージ、偽の `claude` を組み合わせる。着手から完了までを1本のテストで通す
- CIはGitHub ActionsのUbuntuで、`gofmt` の差分、`go vet`、`go test` を実行する。製品のコードと既定の設定に `uskn` が無いことも、テストとして確かめる（ADR-021）

## Risks / Trade-offs

- ［GitHubのGraphQLで編集者の情報が欠ける］→取れないときは「編集者不明」として、信頼済みでない側に判定する
- ［ポーリングの間隔だけ反応が遅れる］→Phase 1では受け入れる。中継はPhase 4で入れる
- ［claude CLIの出力形式が変わる］→解析は `claudecode` に閉じ込め、Phase 0のstream-jsonをテストの入力として残す
- ［同じOSユーザーで動くため、Claudeが `~/.ssh` などに届く］→ADR-023のとおり、対象を非公開リポジトリかつマスターだけが書けるissueに限る
- ［サブスクリプションの利用上限］→`max_parallel` の既定を1とする。上限に達したときの出力の形（V12）は、運用の中で記録する

## Migration Plan

新しく作る機能なので、移行の手順は無い。戻すときは、`switchboard agent run` を止め、状態ラベルを外せば、リポジトリは導入前と同じ状態になる。
