# Tasks

## 1. 外部コマンドの停止

- [x] 1.1 `shell` に `Setpgid`、プロセスグループを殺す `Cmd.Cancel`、`WaitDelay` を設定し、パイプを開いたままの孫プロセスでも時間切れと取り消しで戻るテストが通ることを確かめる

## 2. worktreeとpush前の検査

- [x] 2.1 `CheckClaudeConfig` を分岐点と比べ、ベースが進んだ後も通り、ブランチの変更と未追跡のファイルは検出するテストが通ることを確かめる
- [x] 2.2 ADR-018の検査と保護パスを任意の階層に広げ、`sub/CLAUDE.md` を拒むテストが通ることを確かめる
- [x] 2.3 `CheckOutput` に `--no-renames` を付け、保護パスから外へのリネームを拒むテストが通ることを確かめる

## 3. 信頼の判定とフォージの読み取り

- [x] 3.1 GitHubのコメントをカーソルで全ページ読み、101件以上でも編集者が分かるテストが通ることを確かめる
- [x] 3.2 タイトルの編集者を取り（GitHubの `RenamedTitleEvent`、GitLabの「changed title」）、信頼されない人が変えると着手しないテストが通ることを確かめる
- [x] 3.3 GitLabの本文の編集者を「changed the description」で決め、ノートの上限を外し、件名に「description」を含めても本文の編集者が変わらないテストが通ることを確かめる

## 4. Web表示

- [x] 4.1 `listen` をIPv4射影を戻してから検査し、`[::ffff:0.0.0.0]`、CGNAT、グローバルを拒むテストが通ることを確かめる
- [x] 4.2 すべての要求で `Host` を確かめ、Hostの違うGETを421で断るテストが通ることを確かめる

## 5. 複数の端末と起動

- [x] 5.1 回復処理が、他の端末が実行中のissueを書き戻さないテストが通ることを確かめる
- [x] 5.2 着手の `ack` を、着手のコメントを除いた最新のコメントのIDにし、2台の競合で片方だけが勝つテストが通ることを確かめる
- [x] 5.3 起動時の `RepoPrivate` の失敗を、そのプロジェクトだけのやり直しにし、起動するテストが通ることを確かめる

## 6. 中継、サービス、状態、受信箱

- [x] 6.1 空のwebhookのシークレットを起動時と検証時に拒み、配信IDの無いwebhookをログに残すテストが通ることを確かめる
- [x] 6.2 SSEのIDを起動の時刻から数え、再起動の後のIDが前より大きいテストが通ることを確かめる
- [x] 6.3 `inbox serve` と `switchboard-web` が `Shutdown` の完了を待つようにし、`make verify` が通ることを確かめる（終了の待ちは自動テストが無く、コードを読んで確かめた）
- [x] 6.4 systemdのunitの `Environment=` と `ExecStart=` を引用符で囲み、空白を含むパスのテストが通ることを確かめる
- [x] 6.5 `CredentialName` の衝突を、設定の読み込みと `service install` で誤りにするテストが通ることを確かめる
- [x] 6.6 受信箱の検索範囲を設定のプロジェクトから作り、名前を大文字小文字を区別せずに比べるテストが通ることを確かめる
- [x] 6.7 `state.WriteJSON` を `os.CreateTemp` にし、並行に書いても壊れないことを `-race` で確かめる
- [x] 6.8 `agent.retention` を足し、保持期間を過ぎた実行ディレクトリを消し、最新の実行を残すテストが通ることを確かめる
- [x] 6.9 直っていた2件（取り消しのキャッシュ、ソケットの探索）を、`TestACanceledRequestStillFetches` と `TestClientFindsALaterSocket` で確かめる

## 7. doctorとプロンプト

- [x] 7.1 `doctor` でGitLabのスコープ、classic PATの `X-OAuth-Scopes`、fine-grained PATの読み取りを診断するテストが通ることを確かめる
- [x] 7.2 プロンプトが、GitLabのissueを「GitLab issue」と書くテストが通ることを確かめる

## 8. 文書

- [x] 8.1 公開リポジトリを対象にするADRと、フックを使わないADRを `docs/DECISIONS.md` に足す
- [x] 8.2 AGENTS.md、ARCHITECTURE.md、DESIGN.md、SECURITY.md、ROADMAPを実装に合わせ、`config/switchboard.example.yaml` に `agent.retention` を足す

## 9. テストとCI

- [x] 9.1 `judge` で、GitHubとGitLabが混在する設定の各プロジェクトが自分のフォージのIDだけを信頼するテストを足し、通ることを確かめる
- [x] 9.2 `settings` のallowとdenyを完全一致で確かめるテストを足す
- [x] 9.3 偽フォージに `UpdatedIssues` とPRの作成の失敗の注入を足し、それを使うテスト（PRの失敗で理由付きで待つ、一覧の失敗の後に次のポーリングで拾う）が通ることを確かめる
- [x] 9.4 `make verify` の `verify-test` と `web` を `go test -race` にし、`make verify` が通ることを確かめる
- [x] 9.5 `relayclient` の `Idle` で、無通信の接続を切ってつなぎ直すテストが通ることを確かめる
- [x] 9.6 `harnessfree` の対象に `inboxapi/` と `web/` を加え（`node_modules` と `dist` は除く）、テストが通ることを確かめる
- [x] 9.7 `TestTimeoutSendsINTThenTERMThenKILL` で、偽のスクリプトがtrapを張り終えたことをファイルで確かめる
- [x] 9.8 エージェントと人間が同じアカウントの構成で、ready、waiting、返答、doneを通しで試すテストが通ることを確かめる

## 10. 整理（振る舞いを変えない）

- [x] 10.1 権限規則の文法と `error_max_turns` を `internal/claudecode` に移し、既存のテストと `settings` の完全一致のテストが通ることを確かめる
- [x] 10.2 `SearchOpenIssuesWithLabel` に `forge.Scope` を渡し、`repo:` の組み立てをGitHubのadapterに移したテストが通ることを確かめる
- [x] 10.3 質問の取り出しを `internal/postfmt` に、`relay.Event` を `internal/event` に移し、`inbox`→`agent` と `agent`→`relay` の依存が無いことを確かめる

## 11. Goのツールチェーン

- [x] 11.1 `go.mod` と `web/go.mod` を `go 1.26.0`、`toolchain go1.26.10` にし、`make verify` がGo 1.26.10で通ることを確かめる
