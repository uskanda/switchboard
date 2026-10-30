# ロードマップ

小さく作り、各フェーズの完了条件を満たしてから次へ進む。

## Phase 0 — 検証スパイク

前提にしている Claude Code の挙動を確認する。結果は下の「検証項目」に追記する。

- [x] Stop / Notification フックの入力内容（session_id、cwd、待ち理由が取れるか）
- [x] `claude -p` 実行中に人間の判断が必要になったときの挙動
- [ ] `--resume` の条件と、対話中セッションと重なった場合の挙動（条件は確認済み。重なった場合は未確認）
- [x] 許可ツール・拒否ルールの設定で main への push・マージを確実に止められるか

完了条件：検証項目がすべて「結果」欄つきで埋まっている

## Phase 1 — 最小構成（GitHub のみ・ポーリング・1 リポジトリ）

- [x] 設定ファイル読み込み（対象リポジトリ・端末対応・trusted_authors）
- [x] `switchboard labels sync`
- [x] エージェント：ポーリングで `agent:ready` / 信頼済みコメントを検知 → `claude -p` 実行
- [x] フック：待ち・完了時のコメントとラベル付け替え、隠しマーカー
- [x] `switchboard inbox` / `switchboard reply`

完了条件：issue に `agent:ready` を付けると、待ち→返答→PR 作成まで手でコマンドを打たずに回る

## Phase 2 — GitLab 対応・複数リポジトリ

- [ ] GitLab クライアント
- [ ] 受信箱の GitHub / GitLab マージ表示
- [ ] 複数リポジトリ・worktree 運用

完了条件：両フォージの待ちが 1 つの一覧に出て、どちらにも返答できる

## Phase 3 — 複数端末

- [ ] 端末ごとの担当設定、同一 issue の重複実行防止
- [ ] 常駐化（macOS: launchd、Linux: systemd、または tmux）

完了条件：2 台以上の端末で、それぞれの担当 issue だけが処理される

## Phase 4 — 中継

- [ ] webhook 受信・署名検証・重複排除
- [ ] ID 通知のみの SSE 配信
- [ ] エージェントの中継モード＋低頻度ポーリングでの照合

完了条件：返答から実行開始までの遅延がポーリング間隔に依存しなくなる。中継停止時もポーリングで継続する

## Phase 5 — 使い勝手

- [ ] 受信箱のローカル Web 表示（スマホから見る用）
- [ ] 待ち発生時のプッシュ通知
- [ ] 運用ログからのループ改善（失敗パターンの振り返り）

## 検証項目

確認の条件：クラウド環境（Claude Code on the webのコンテナ）、`claude` 2.1.312、モデルはhaiku。`claude -p` は、許可リストで組み立てた環境変数（`env -i` で `HOME`・`PATH`・プロキシの設定だけを渡す）で起動した。記録は `add-phase1-minimal` の作業中に取った。

| # | 項目 | 結果 | 確認日 |
|---|---|---|---|
| V1 | Stop / Notification フックで session_id・cwd が取れるか | 取れる。Stopの入力には `session_id`・`cwd`・`transcript_path`・`permission_mode`・`stop_hook_active` が入る。`last_assistant_message` は、最後の出力がテキストのときだけ入り、`--json-schema` の構造化出力で終わったときは入らない | 2026-10-29 |
| V2 | Notification フックで「権限確認待ち」と「入力待ち」を区別できるか | `-p` ではNotificationフックが一度も発火しなかったため、区別は確かめられない（V13） | 2026-10-29 |
| V3 | `claude -p` で判断が必要になった場合、停止するか失敗するか | 停止も失敗もしない。許可の無いツール呼び出しは拒否され、実行は続く。拒否された呼び出しはresultの `permission_denials` に残る | 2026-10-29 |
| V4 | 対話中セッションと `--resume` が重なった場合の挙動 | 重なった場合は未確認。`--fork-session` を付けると新しいセッションIDで再開されるため、設計では常に分岐させて重なりを避ける | 2026-10-29 |
| V5 | 権限設定で main への push・マージを拒否できるか | 確実には拒否できない。`Bash(git push*)` のdenyは `git push` を直接呼ぶ形だけを止め、`sh -c 'git push ...'` はすり抜けてpushされた | 2026-10-29 |
| V6 | サブスクリプション認証でのヘッドレス常駐実行に制約がないか | 未確認。マスターの端末で確かめる | |
| V7 | `-p` で `--session-id <uuid>` を指定できるか | 指定できる。resultとフックの `session_id` に指定したIDが入る。`--resume <id> --fork-session` は新しいIDを返し、別の作業ディレクトリからも再開できた | 2026-10-29 |
| V8 | `--json-schema` の `structured_output` がstream-jsonのresultに入るか | 入る。Claudeは `StructuredOutput` ツールで出力し、resultの `structured_output` にスキーマどおりのオブジェクトが入る | 2026-10-29 |
| V9 | `--settings` のフック・権限・サンドボックス設定が `-p` で適用されるか | フックと権限は適用される。サンドボックスは、このコンテナに `bwrap` が無いため確かめられない。`-p` でも依存の不足を警告し、サンドボックスなしで続行した | 2026-10-29 |
| V10 | 専用OSユーザーと `CLAUDE_CODE_OAUTH_TOKEN` でヘッドレス実行が回るか | 未確認。マスターの端末で確かめる | |
| V11 | `permission_denials` の要素の形 | 要素は `tool_name`・`tool_use_id`・`tool_input` を持つ。Bashなら `tool_input.command` に拒否されたコマンドが入る | 2026-10-29 |
| V12 | 利用上限に達したときの終了コード・`subtype`・出力の形 | 未確認。意図して再現する方法が無いため、Phase 1の運用中に記録する | |
| V13 | `-p` 実行中にNotificationフックが発火するか | 発火しなかった。`dontAsk` と `default` の権限モードの両方で確かめた | 2026-10-29 |
| V14 | GitHub GraphQL searchでissueと直近コメントを1リクエストで取れるか | 未確認。Phase 1の受信箱は、DESIGN.md 4.6節の想定と違い、RESTのSearch API（`/search/issues`）を1回呼び、待ちのissueごとにGraphQLでスレッドを取る。実物のGitHubで、この形で動くことはV20で確かめた。Search APIには30回/分の制限があるため、GraphQLの1リクエストで取る形に変えるかは、自動更新するPhase 5のWeb表示の前に決める | 2026-10-30 |
| V15 | Workflows権限なしのfine-grained PATで、ワークフロー変更を含む `git push` が拒否されるか | 未確認。マスターの端末で、検証用リポジトリとPATを使って確かめる | |
| V16 | 利用するGitLabでwebhookの署名トークンが使えるか | 使える。`gl.uskan.dev`（CE 19.5.1）で、`POST /projects/:id/hooks` の `signing_token`（`whsec_` に32バイトの鍵のbase64を続けた形）で設定できた。応答には `signing_token_present: true` だけが返る。届いたIssue HookとNote Hookには `webhook-id`、`webhook-timestamp`、`webhook-signature`（`v1,<base64>`）が付き、`webhook-id` は `Idempotency-Key` と同じ値だった。署名は、`whsec_` を除いてbase64を解いた鍵による、`<webhook-id>.<webhook-timestamp>.<ボディ>` のHMAC-SHA256と一致した。Note Hookでは、issueの番号が `issue.iid` に、種類が `object_attributes.noteable_type`（`Issue`）に入る。`127.0.0.1` 宛ては、ローカルへの送信の許可リストに `127.0.0.1:8781` を足すまで、登録の時点で `Invalid url given` になった。足したあと、Sidekiqから送るイベントは数分の間 `BlockedUrlError` で失敗し、20分後には届いた。同期で送るテストと再送は、すぐに届いた | 2026-10-30 |
| V17 | `--setting-sources` で対象リポジトリの `.claude/settings.json` を読み込ませずに済むか | 済む。`--setting-sources user` ではプロジェクトのフックが動かなかった。ただし、プロジェクトの `CLAUDE.md` も読み込まれなくなった | 2026-10-29 |
| V18 | `--permission-mode dontAsk` の `-p` での挙動 | allowに無いツール呼び出しを、確認せずに拒否して続行する。PermissionRequestフックは発火しない。ただし `uname -a` のような読み取り専用のコマンドは、allowに無くても実行された | 2026-10-29 |
| V19 | Phase 1の完了条件（`agent:ready` から待ち、返答、PRの作成まで手で操作せずに回るか）を、実物の `claude` で満たすか | 満たす。偽のフォージと実物のgitリポジトリに対し、claude 2.1.312（モデルは既定）で、質問、受信箱からの返答、完了、draftのPRの作成まで回った。1回目は5秒、2回目は11秒だった。`go test -tags realclaude ./internal/integration/` で再現できる。実物のGitHubに対する確認は、マスターの端末で行う | 2026-10-29 |
| V20 | Phase 1の完了条件を、Ubuntuの端末と実物のGitHubで満たすか | 満たす。Ubuntu 24.04の端末で、非公開リポジトリ `uskanda/switchboard-sandbox`、Secret Service（gnome-keyring）のfine-grained PAT 2つ、claude 2.1.314（モデルは既定）を使った。issueに `agent:ready` を付けると、16秒で質問が付いて `agent:waiting` になった。`switchboard reply` で返答すると、16秒後に再び実行し、31秒で `switchboard/issue-1` からdraftのPRを作って `agent:done` になった。mainは変わらなかった。初回のポーリングで `since` にゼロ値の時刻を送り、GitHubが422で拒否する不具合が見つかった。偽のフォージでは再現しないため、結合テストでは分からなかった。ゼロ値のときは `since` を付けないよう直した | 2026-10-30 |
| V21 | GitLabで、issue本文とノートの最後の編集者が取れるか | 取れる。`gl.uskan.dev`（CE 19.5.1）で確かめた。本文を編集すると、編集した人を作成者とするシステムノート（本文は `changed the description`）が付く。ノートは、GraphQLの `lastEditedBy` に、編集したときだけ編集者が入る。`lastEditedAt` は編集していなくても入るため、編集の有無には使えない。issueのGraphQLには `lastEditedBy` が無い | 2026-10-30 |
| V22 | GitLabで、ラベルを付けた人と更新を取れるか | 取れる。`resource_label_events` の `add` の `user` に、ラベルを付けた人が入る。`updated_after` で更新を取れる。ただし、ノートを足してもissueの `updated_at` は変わらない。ラベルを変えると変わる。そのため、返答の検知には、待ちのissueの `user_notes_count` を毎回見る | 2026-10-30 |
| V23 | GitLabで、draftのMRを作れるか | 作れる。タイトルの先頭に `Draft: ` を付けて `POST /projects/:id/merge_requests` を呼ぶと、`draft` が真のMRになる。`add_labels` に無いラベルを渡すと、そのラベルはプロジェクトに作られる | 2026-10-30 |
| V24 | Phase 2の完了条件（両フォージの待ちが1つの一覧に出て、どちらにも返答できるか）を、実物で満たすか | 満たす。usknasで、GitHubの `uskanda/switchboard-sandbox` とGitLab（`gl.uskan.dev`、CE 19.5.1）の `nas/switchboard-sandbox` に1つずつissueを立てた。claude 2.1.314で、両方が質問を残して待ちになった。`switchboard inbox` の1つの一覧に両方が出て、`--forge gitlab` でGitLabだけに絞れた。受信箱から両方に返答すると、2分以内に両方が完了し、GitHubにdraftのPR、GitLabにdraftのMRができた。GitLabの返答は、ノートの数で検知された。どちらのmainも変わらなかった。GitLabのissueを閉じると、次のポーリングでworktreeとローカルのブランチが消え、リモートのブランチは残った。GitLabのトークンは `api` スコープの従来のPATで、HTTPSのクローンに資格情報ヘルパーは無い。fetchとpushは、エージェントが渡したトークンで通った | 2026-10-30 |
| V25 | systemdのサービスとして常駐させたエージェントが、issueを処理し、落ちても再起動するか | 確かめた。usknas（Ubuntu 24.04、systemd 255）で `switchboard service install` を実行すると、unitとクレデンシャルが置かれ、`is-enabled` は `enabled`、`is-active` は `active` になった。`kill -9` でエージェントを止めると、30秒後にsystemdが再起動した。サービスのエージェントは、トークンをクレデンシャルから読み、`claude`（サブスクリプションの認証）を動かした。GitHubのissueで、着手、質問、受信箱からの返答、draftのPRの作成まで回った。両フォージの設定で設置し直すと、`systemctl restart` でエージェントが正常に止まり、すぐに起動し直した。GitLabのissue（`nas/switchboard-sandbox` #4）も、着手のコメント、完了のコメント、draftのMR（!3）の作成まで、サービスで回った。2台での同時の動作は、#9で確かめる | 2026-10-30 |

### 検証で分かった、設計に関わる点

- 親のClaude Codeのセッションから起動すると、子の `claude -p` は環境変数 `CLAUDE_CODE_SESSION_ID` を引き継ぎ、親と同じセッションIDで動いた。エージェントは環境を許可リストで組み立てる設計（DESIGN.md 4.4節）なので、この変数は渡さない
- `--setting-sources user` ではプロジェクトの `CLAUDE.md` も読み込まれない（V17）。ADR-021はプロジェクト設定を読み込まないと決めているため、リポジトリの指示をプロンプトで渡す方法か、ADR-018の検査を前提にプロジェクト設定を読み込む方法かを、Phase 1の成果物で決める
- `default` の権限モードでは、承認の要る呼び出しでPermissionRequestフックが動き、判定を返さなければ拒否された。`dontAsk` ではこのフックが動かないため、拒否の記録は `permission_denials` から取る
- denyルールは境界にならない（V5）。Claudeのプロセスにフォージの資格情報を渡さないADR-012が、push対策の本体になる
