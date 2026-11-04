# Design

## Context

監査の指摘は約45件ある。2件（`inbox serve` の取り消しのキャッシュ、`switchboard-web` のソケットの探索）は、mainで直っていた。既存のテスト（`TestACanceledRequestStillFetches`、`TestClientFindsALaterSocket`）で確かめ、チェックを付ける。残りは今も再現した。決定はgrilling.mdにある。

## Goals / Non-Goals

**Goals:**
- 指摘をすべて1本のPRで直す。コミットは項目のまとまりごとに分け、振る舞いを変えない整理は後ろに置く
- 新しい振る舞いには、先に失敗するテストを書く

**Non-Goals:**
- `CredentialName` の付け方を変えること（grilling Q5）
- Web表示でTailscaleのアドレス（`100.64.0.0/10`）を許すこと（grilling Q4）

## Decisions

- **外部コマンドの停止**：`Setpgid` で独立したプロセスグループにし、`Cmd.Cancel` でグループ全体を殺す。`WaitDelay` で、孫プロセスがパイプを開いたままでも `Wait` を返す
- **ADR-018の比べ先**：`git merge-base HEAD origin/<base>`。ブランチを書き換えず、衝突も起きない。直す実行が `git merge origin/<base>` を取り込めば、分岐点も進む
- **下の階層**：pathspecを `:(glob)**/CLAUDE.md` などにし、保護パスはパスの要素で比べる。`.mcp.json` はルートだけ
- **リネーム**：`git diff --name-only --no-renames` で、元のパスも検査に入れる
- **全ページの読み取り**：GitHubはGraphQLのカーソルで、GitLabはノートの上限を外して読む。タイトルの編集者は、GitHubは `RenamedTitleEvent` の `actor`、GitLabは「changed title」のシステムノートで取る
- **着手の判定**：`ack` を、エージェントの着手のコメントを除いた最新のコメントのIDにする。相手の着手を読んだ後でも、両方が同じ `ack` を比べる
- **SSEのID**：起動の時刻（マイクロ秒）から数え始める
- **検索の範囲**：`SearchOpenIssuesWithLabel` は `forge.Scope`（プロジェクトの一覧と、GitHubの `search_scope`）を受け取る。`repo:` の組み立てと、GitLabのプロジェクトごとの読み取りは、各adapterに閉じる。範囲が空ならGitHubに問い合わせない
- **依存の整理**：質問の取り出しを `internal/postfmt` に、`relay.Event` を `internal/event` に移す。`relay.Event` は `event.Event` の別名として残し、中継の内部はそのまま使う。パッケージ名を `post` にしなかったのは、`agent` と `inbox` のテストに同名の型があるため
- **Go**：`go 1.26.0` と `toolchain go1.26.10`。Ubuntu 26.04 LTSのどの1.26でもビルドでき、`GOTOOLCHAIN=auto` とCIは1.26.10を使う。2026-11-04の時点で1.26.11は未公開だった

## Risks / Trade-offs

- `TestTimeoutSendsINTThenTERMThenKILL` の時間切れを2秒に延ばしたので、テストが約2秒長くなる。偽のスクリプトがtrapを張り終えたかをファイルで確かめ、張る前に時間切れが来たときは、そのことを示して失敗する
- この端末のsandboxはネットワークを使えないので、`go mod tidy` と `govulncheck` を実行できなかった。`make verify` は、読み取り専用のモジュールのモードで1.26.10のビルドとテストを通した
