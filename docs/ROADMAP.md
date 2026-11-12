# ロードマップ

小さく作り、各フェーズの完了条件を満たしてから次へ進む。

## Phase 0 — 検証スパイク

前提にしている Claude Code の挙動を確認する。結果は下の「検証項目」に追記する。

- [x] Stop / Notification フックの入力内容（session_id、cwd、待ち理由が取れるか）
- [x] `claude -p` 実行中に人間の判断が必要になったときの挙動
- [ ] `--resume` の条件と、対話中セッションと重なった場合の挙動（条件は確認済み。重なった場合は未確認）
- [x] 許可ツール・拒否ルールの設定で main への push・マージを確実に止められるか（止められない。V5。防壁はsandboxとpush前の検査とした、ADR-063）

完了条件：検証項目がすべて「結果」欄つきで埋まっている

## Phase 1 — 最小構成（GitHub のみ・ポーリング・1 リポジトリ）

- [x] 設定ファイル読み込み（対象リポジトリ・端末対応・trusted_authors）
- [x] `switchboard labels sync`
- [x] エージェント：ポーリングで `agent:ready` / 信頼済みコメントを検知 → `claude -p` 実行
- [x] 書き戻し：待ち・完了時のコメントとラベル付け替え、隠しマーカー（フックではなく、`claude -p` の終了後にエージェントが行う。ADR-011、ADR-063）
- [x] `switchboard inbox` / `switchboard reply`

完了条件：issue に `agent:ready` を付けると、待ち→返答→PR 作成まで手でコマンドを打たずに回る

## Phase 2 — GitLab 対応・複数リポジトリ

- [x] GitLab クライアント
- [x] 受信箱の GitHub / GitLab マージ表示
- [x] 複数リポジトリ・worktree 運用

完了条件：両フォージの待ちが 1 つの一覧に出て、どちらにも返答できる（V24で満たした）

## Phase 3 — 複数端末

- [x] 端末ごとの担当設定、同一 issue の重複実行防止（着手のコメントによる判定）
- [x] 常駐化（Linux の systemd、`switchboard service install`。macOS の launchd は作っていない）

完了条件：2 台以上の端末で、それぞれの担当 issue だけが処理される

## Phase 4 — 中継

- [x] webhook 受信・署名検証・重複排除
- [x] ID 通知のみの SSE 配信
- [x] エージェントの中継モード＋低頻度ポーリングでの照合

完了条件：返答から実行開始までの遅延がポーリング間隔に依存しなくなる。中継停止時もポーリングで継続する（V26で満たした）

## Phase 5 — 使い勝手

- [x] 受信箱のローカル Web 表示（スマホから見る用）
- [x] 待ち発生時のプッシュ通知（Telegram、AndroidのアプリへのFCM）
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
| V12 | 利用上限に達したときの終了コード・`subtype`・出力の形 | 運用中の実行ログ（`runs/*/stream.jsonl`）で確かめた。まず `rate_limit_event` が出る。`rate_limit_info.status` は `rejected`、`resetsAt` は解除時刻のUNIX秒、`rateLimitType` は `five_hour` である。続くresultは `subtype` が `success` のままで、`is_error: true`、`api_error_status: 429`、`api_error: usage_limit_reached`、`terminal_reason: api_error` を持ち、本文は「You've hit your 5-hour limit · resets 6:30pm (UTC)」だった。手前では `status` が `allowed_warning` のイベントも出る（ADR-053では使わない）。伏せ字にした出力を `internal/claudecode/testdata/limit.jsonl` に置いた。自動の再開を本番で確かめたら、ここに書き足す | 2026-11-03 |
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
| V26 | Phase 4の完了条件（返答から実行の開始までの遅延がポーリングの間隔に依存せず、中継が止まってもポーリングで処理が続くか）を、実物で満たすか | 満たす。usknasで、`switchboard-relay.service` と、中継モードのエージェントのサービスを動かした。GitLab（`gl.uskan.dev`、CE 19.5.1）の `nas/switchboard-sandbox` のwebhookは、`relay hook add` でAPIから登録した。中継につながっている間は、issueの作成から着手までが4.2秒だった。受信箱からの返答から再実行の着手までは4.6秒で、どちらも3秒のまとめ待ちを含む。ポーリングの間隔は30秒である。中継を止めると、エージェントはすぐに切断を記録し、ポーリングに戻った。その間の返答は、25.6秒後のポーリングで検知されて完了した。中継を起動し直すと、エージェントは再接続の間隔（最大60秒）の後に再接続し、照合を1回行った。止めている間の5回の配信は失敗したが、webhookは止められず（`alert_status` は `executable`）、起動し直したあとの配信は届いた。ボットのアカウントで立てたissueは、ラベルと本文が信頼できない人のものとして拒否された | 2026-10-31 |
| V27 | Phase 5の確認（待ちと完了がTelegramに届き、受信箱のWeb表示から返答して実行が再開するか）を、実物で満たすか | 満たす。usknasで、エージェント（中継モード、Telegramの通知あり）、`switchboard-inbox.service`、`switchboard-web.service` を動かした。GitLabの `nas/switchboard-sandbox` #8が質問を残して待ちになると、Telegramに待ちの通知が届いた。通知のリンクからスマホのブラウザでWeb表示を開き、返答を送ると、`uskanda` のコメントとして投稿された。中継で4.1秒後に再実行に着手し、14秒後に完了してMRを作った。完了の通知も届いた。デスクトップのブラウザでも、一覧と1件の画面を確かめた。Web表示の8783番は、firewalldの `public` ゾーンで、LAN（192.168.0.0/24）からだけ許す規則を足すまで届かなかった。スマホのChromeとTelegramのアプリ内ブラウザでは、Web表示もgl.uskan.devも429（Too Many Requests）で開けず、他のブラウザでは開けた。Web表示とNASは429を返さないので、ブラウザの側の原因と見ている。設置の直後に見た空の一覧が60秒キャッシュされ、待ちになってから一覧に出るまでに最大で1分かかった | 2026-10-31 |
| V28 | GitHubの検索で、`label:"agent:waiting","agent:running"` がラベルのORになるか | なる。`gh api search/issues` で `uskanda/switchboard` を調べた。`label:"agent:done"` は1件、`label:"agent:waiting"` は1件で、`label:"agent:done","agent:waiting"` は2件だった。受信箱は、GitHubの待ちと実行中を1回の検索で取る | 2026-10-31 |
| V29 | 計画の実行の権限（`deny` に `Edit`、`Write`、`NotebookEdit`）で、実物の `claude` がファイルを書けるか | 書けない。`deny` に入れたツールは、拒否されるのではなく、セッションに渡されなかった（initの `tools` に無い）。claude 2.1.316で、プロンプトで `Write` での書き込みを直接頼んでも、モデルは「Writeツールが無い」と答えて、`plan` を宣言した。Bashでの迂回もしなかった。ただし `Bash` は渡されるため、リダイレクトでの書き込みは権限では止まらない。実行のあとのworktreeの検査を防壁とする（ADR-038）。`go test -tags realclaude -run RealClaudePlanSettingsRefuseWrites ./internal/agent/` で再現できる | 2026-10-31 |
| V30 | 計画を先に立てるモードが、実物の `claude` で、質問、計画、承認、実装の順に回るか | 回る。偽のフォージと実物のgitリポジトリに対し、claude 2.1.316で、`agent:plan` を付けたissueを動かした。1回目の計画の実行（9秒）は、推奨の答え付きのQ1だけを聞いた。`/ok The word is hi.` の返答で、2回目（9秒）は推奨と違う答えを反映した計画を出した。計画への `/ok` で、承認のあとの実行（12秒）が `greeting.txt` を作り、検証を通ってdraftのPRができた。2回の計画の実行のあと、worktreeに変更もコミットも無かった。`go test -tags realclaude -run RealClaudePlansBeforeItImplements ./internal/integration/` で再現できる。実物のGitHubと常駐のサービスでの確認は、別に行う | 2026-10-31 |
| V31 | 計画を先に立てるモードを、usknasの常駐のサービスと実物のGitHubで回せるか | 回せる。`abf62c6` の版に入れ替えて `labels sync` を実行し、`uskanda/switchboard-sandbox` #7に `agent:plan` だけを付けた。claude 2.1.316で、1回目の計画の実行は推奨の答え付きのQ1を聞いた（着手から19秒）。CLIの `/ok q1はB` の返答で、上書きを反映した計画が出た。そのあと受信箱のWeb表示の保存の口から、プロジェクトの追加の指示を保存した。`Origin` の無いPOSTは403、トークンを含む内容は422で、保存は監査ログにSHA-256だけを残した。Web表示から計画に直しを返すと、計画が出し直され、その実行のsystem promptに追加の指示の節が入り、計画の最後に指示どおり「確かめ方」が付いた。Web表示から `/ok` を送ると、承認のあとの実行が計画と承認のコメントをIDで名指しされて実装し、draftのPR（#8）を作って `agent:done` になった。mainは変わらなかった。計画の実行には `Write`、`Edit`、`NotebookEdit` が渡されなかった。3つのサービスを同時に再起動すると、Web表示が受信箱のソケットより先に起動して、ソケットの場所を空のまま決めた。Web表示だけを再起動し直すと直った（既存の不具合） | 2026-10-31 |
| V32 | WUIから足したプロジェクトを、usknasの常駐のサービスが再起動せずに扱い始めるか | 扱い始める。`add-web-project-add` の版にし、設定から `gh:uskanda/switchboard-sandbox` を外して `inbox.project_root: ~/repos` を足した。元のcloneは退避し、sandboxから `agent:go` ラベルを消した。サービスを起動し直したあと、Web表示の設定の画面から `gh:uskanda/switchboard-sandbox` を足した。設定ファイルには、`base_branch: main` と `plan_first: true` を含む5行が `projects` の最後に差し込まれた。`config.yaml.bak` が残り、ほかの行は変わらなかった。追加から11秒で、エージェントがcloneし、`agent:go` を作り直した。直後に `agent:go` で立てたissue #9は、ポーリングで着手され、30秒でdraftのPR（#10）を作って `agent:done` になった。mainは変わらなかった。issueを閉じると、次のポーリングでworktreeとローカルのブランチが消えた。サービスが動いている間に設定から外したプロジェクトは、再起動せずに扱われなくなった。`inbox` の節の変更は、両方のサービスが「再起動で反映する」とログに出し、反映しなかった | 2026-11-01 |
| V33 | 開始ラベルを付けて作ったissueで、作った人がそのラベルを付けた人として残るか（GitLabは `resource_label_events` のadd、GitHubはtimelineの `labeled`）。残らなければ、エージェントの `LabelActor` が開始ラベルを付けた人を見つけられず、着手しない。その場合はラベルなしで作り、続けて `SetLabels` で付ける2段の形にする | 未確認（`add-web-issue-create` の実装の時点。実物のフォージで確かめる） | |
| V35 | fine-grained PATで、`/repos/<owner>/<repo>/commits/<sha>/check-runs` を読めるか（`Checks` の権限を付けたトークンと付けないトークンで比べる）。読めれば、GitHub Actions以外のアプリが付けるcheck runも、CIの判定に足す | 未確認（`handle-push-and-ci-failures` の実装の時点。今はActionsのjobとcommit statusだけで判定する） | |
| V36 | `Actions: Read` と `Commit statuses: Read` を足したfine-grained PATで、workflow run、job、jobのログ（302のリダイレクトの先）、combined statusを読めるか。足さないトークンでは403になるか | 未確認（`handle-push-and-ci-failures` の実装の時点。sandboxのリポジトリで確かめる） | |
| V37 | usknasの常駐のサービスで、わざと失敗するテストを含むissueのPRのCIが失敗したとき、直す実行が同じPRにpushし、CIが通って見張りが終わるか。人が `switchboard/issue-<n>` にコミットを足してから再実行させると、`git merge` で取り込んでpushされるか | 未確認（`handle-push-and-ci-failures` の実装の時点） | |
| V34 | usknasの常駐のサービスのWUIから `gh:uskanda/switchboard-sandbox` に `agent:plan` でissueを作ると、エージェントが受信箱のトークンの身元を信頼済みと判断し、計画から着手するか。`Origin` の無いPOSTは403になるか | 未確認（`add-web-issue-create` の実装の時点。常駐のサービスで確かめる） | |
| V38 | GitHubのfine-grained PATで、GraphQLの `markPullRequestReadyForReview` と、`sha` を付けた `PUT /repos/<owner>/<repo>/pulls/<n>/merge` が通るか。Pull requestsとContentsの書き込みが無いトークンでは403になるか（GraphQLは200で `FORBIDDEN` の誤りを返すか）。`sha` が違えば409、マージできなければ405になるか | 未確認（issue #69の実装の時点。実装は、409をheadの移動、405と422をフォージの拒否として扱う） | |
| V39 | GitLabで、タイトルから `Draft: ` を外す `PUT /projects/:id/merge_requests/:iid` と、`sha` を付けた `PUT /projects/:id/merge_requests/:iid/merge` が通るか。`sha` が違うとき、マージできないときの応答のコードは何か（405、406、409） | 未確認（issue #69の実装の時点。実装は、409をheadの移動、405と406と422をフォージの拒否として扱う） | |
| V40 | usknasの常駐のサービスのWUIから、`gh:uskanda/switchboard-sandbox` のdraftのPRをマージすると、PRの `Closes` でissueが閉じ、受信箱から消えるか | 未確認（issue #69の実装の時点。受信箱のトークンにREADMEの手順で権限を足してから確かめる） | |
| V41 | `--permission-mode dontAsk` の `claude -p` で、allowに `Skill` が無いときにSkillツールを呼べるか | スキルによる。`allowed-tools` の無いスキルは、allowに `Skill` が無くても呼べた。`allowed-tools` を持つスキルは拒否され、`permission_denials` に `Skill` が残った。allowに `Skill` を足すと呼べた。claude 2.1.316で確かめた。`go test -tags realclaude -run RealClaudeSkillAllow ./internal/agent/` で再現できる | 2026-11-02 |
| V42 | allowの `Bash(git *)` とdenyの `Bash(git push *)` が重なるとき、`git push` が拒否されるか | 拒否される。denyがallowより優先され、`permission_denials` に記録された。リモートには何も届かなかった。ただし、denyは境界にならない（V5）。`go test -tags realclaude -run RealClaudeDenyWinsOverAllow ./internal/agent/` で再現できる | 2026-11-02 |
| V43 | 受信箱のサービスが送るdataだけのFCMのメッセージ（`title`、`message`、`body`、`categoryId`、`channelId`）を、`expo-notifications` がカテゴリのボタン付きで表示するか。アプリが裏にいるとき、終了しているときの両方で確かめる。出なければ、バックグラウンドのタスクで受けてローカルの通知として出す（design D5） | 未確認（`add-android-app` の実装の時点。実機のAndroidで確かめる） | |
| V44 | 終了しているアプリで、計画の承認待ちの通知の「承認」を押すと、`registerTaskAsync` のタスクが動き、承認の語が投稿されるか。動かなければ、「承認」をアプリを開く形に変える（design D6） | 未確認（`add-android-app` の実装の時点。実機のAndroidで確かめる） | |
| V45 | GitHubで、衝突のあるPRのマージが405で断られた直後に、`GET /repos/:owner/:repo/pulls/:number` が `mergeable=false` と `mergeable_state=dirty` を返すか（`null` で計算中のままにならないか）。GitLabで、衝突のあるMRのマージの拒否のあとに `has_conflicts=true` が返るか | 未確認（issue #119の実装の時点。switchboardの実行からはフォージに届かない。計算中で衝突と分からないときは、今までどおり `not_mergeable` を返し、人が押し直す） | |
| V46 | Claude Codeのsandboxは、書き込みを拒否するパスに何を置くか。コマンドが強制終了したとき、それは残るか | 0バイトで権限0444の通常のファイルを置く。worktreeでは `.claude/settings.json` や `.mcp.json`、`.bashrc` など、クローンでは `.git/config.lock` に置く。実行中にエージェントが止まると消されずに残り、worktreeの作成と設定の検査が失敗した（issue #128。usknasのエージェントの実行で観察） | 2026-11-03 |
| V47 | エージェントのfine-grained PAT（Contents: Read/Write、Issues: Read/Write）を `GH_TOKEN` で渡し、空の `GH_CONFIG_DIR` で `gh issue comment -R owner/repo --attach` を実行すると、画像が `user-attachments` のURLで付くか。付いた画像が本文のどこに入るか | 未確認（issue #94の実装の時点。無人の実行からはフォージに届かないため）。`SWITCHBOARD_GH_TOKEN=... SWITCHBOARD_GH_ISSUE=owner/repo#n go test -tags realgh -run RealGH -v ./internal/forge/github/` で確かめる。通らなくても、doneは保たれ、理由のコメントが付く | 確認済み（2026-11-12）。エージェントのトークンと空の `GH_CONFIG_DIR` で、`gh` 2.103の `--attach` が画像を付けた（`gh:uskanda/smarternews` #118、#146など）。画像は `https://github.com/user-attachments/assets/<uuid>` のURLで、本文の末尾に並ぶ。公開リポジトリでは、このURLを認証なしで読める |
| V48 | GitLabの `POST /projects/:id/uploads` が返す `markdown` をissueのノートに入れると、画像として表示されるか | 未確認（issue #94の実装の時点）。`gl:` のsandboxのプロジェクトで、画像を置く実行をdoneまで進めて確かめる | 確認済み（2026-11-12）。`gl:nas/scripts#26` の計画の質問で、アップロードの `markdown` を本文の途中に置き、GitLabの画面で画像として出た（V58） |
| V49 | `-p` に `--tools` を渡すと、セッションのツールはその一覧に絞られるか。`--json-schema` の構造化出力は動くか | 絞られる。initの `tools` は渡した一覧と `StructuredOutput` だけになり、構造化出力は動いた。`Bash,Read,Edit,Write,Glob,Grep` で、起動直後の文脈は約39kから約9kトークンに減った。`allow_tools_extra` の `Skill` を足すと `Skill` も渡った。claude 2.1.316。`go test -tags realclaude -run RealClaudeRunGetsOnlyItsTools ./internal/agent/` で再現できる | 2026-11-03 |
| V50 | `-p` で `--model` と `--effort` は効くか。`--setting-sources project,local` で何が外れるか | `--model sonnet` はinitの `model` に反映された。`--effort` は受け付けられるが、initとresultに値は出ない。`--setting-sources project,local` では、ユーザーのスキル、プラグイン、model設定が外れ、文脈は約3kトークン減った。`~/.claude/CLAUDE.md` は外れなかった。claude 2.1.316 | 2026-11-03 |
| V51 | `--setting-sources project,local` と `--settings` のsandboxを組み合わせた `-p` で、sandboxは働くか。unixソケット、自動の許可、`failIfUnavailable` はどう振る舞うか | 働く。worktreeの外への書き込みは `Read-only file system` で失敗し、ネットワークへの接続はプロキシに拒否された。sandboxの中ではunixソケットを開けず、`excludedCommands` のコマンドは外で動いてinboxserveのテストが通った。外で動くコマンドにも許可の規則が要る。`autoAllowBashIfSandboxed` が有効だと、規則の無いコマンドも動き、拒否の規則は働いた。`failIfUnavailable: true` では、bwrapかsocatが無いとclaudeは起動しない。名前空間を作れない環境ではclaudeは起動し、Bashがすべて失敗する。`bwrap --ro-bind / / --dev /dev --unshare-all true` はその環境で1、作れる環境で0を返した。claude 2.1.316 | 2026-11-03 |
| V52 | `excludedCommands` の `make verify` は、`make deploy` も外で動かすか。`denyRead` に無いパスを渡すと、sandboxはそこにファイルを置くか | 動かさない。`make verify` はsandboxの外で動き、`make deploy` は中で動いてworktreeの外に書けなかった。無いパスを渡しても、sandboxはそこにファイルを置かなかった。同じ実行で、規則の無い `python3` は動き、`gh` は拒否された。worktreeの外への書き込みは失敗し、`denyRead` のファイルは読めなかった。claude 2.1.316。`go test -tags realclaude -run RealClaudeSandbox ./internal/agent/` で再現できる | 2026-11-03 |
| V53 | `--resume <A> --fork-session` に `--session-id <B>` を併せたとき、分岐先のIDは `B` になるか。再開した実行で、ツール、スキーマ、追記するシステムプロンプトは新しい値になるか。無いセッションを再開すると何が返るか | 分岐先のIDは `B` になった。`--tools` と `--json-schema` は新しい値が使われた。`--append-system-prompt-file` は使われず、最初の実行のシステムプロンプトのままだった。キャッシュの有効期間は1時間で、ツールとシステムプロンプトが同じなら前回の履歴がキャッシュから読まれた。ツールを変えると読まれなかった。無いセッションでは、終了コード1で、resultは `error_during_execution`、`num_turns` は0、`errors` は `No conversation found with session ID: <A>` だった。claude 2.1.316 | 2026-11-03 |
| V55 | 受信箱のWeb表示から、実物のGitLabのOpenID Connectでログインできるか。`/oauth/userinfo` の `groups` に、所属するグループと下位のグループのフルパスが入るか。グループから外した人が、トークンの更新のあとに締め出されるか | 未確認（`add-web-oidc-login` の実装の時点。usknasのGitLabで確かめる） | |
| V56 | 認証を設定した受信箱で、Androidのアプリの外の `fetch`（端末の登録と、通知からの承認）に、WebViewのセッションのcookieが付くか。付かなければ、端末の登録をWebViewの画面から行う形に変える | 未確認（`add-web-oidc-login` の実装の時点。実機のAndroidで確かめる） | |
| V57 | 実物のGitLabで、ログインした2人のmemberのそれぞれの返答が、その人のコメントとして残るか。OAuthのアクセストークン（`Authorization: Bearer`）で、コメント、マージ、issueの作成ができるか。読めないプロジェクトが一覧に出ないか | 未確認（`add-web-multi-user` の実装の時点。usknasのGitLabで確かめる） | |
| V58 | Claude Codeのsandboxの中の1回のBashの呼び出しで、開発サーバーを起動し、`switchboard shot` でChromiumを動かして、そのページを撮れるか。ループバックの外への読み込みは止まるか | 撮れる。sandboxの中でも、Chromiumは `--no-sandbox` で起動した。同じ呼び出しで起動した `python3 -m http.server` のページを、注記とDOMの書き換えを付けて撮れた。`127.0.0.2` のサーバーへの画像、fetch、WebSocketの要求は、存在しないプロキシへ送られて届かなかった。Chromiumの暗黙のループバックの除外（`127.0.0.0/8`）は、`<-loopback>` で外す必要があった。確かめたのは対話のセッションのBashのsandboxである。計画の実行の `claude -p`（`dontAsk`）では、`$!`、`$(...)`、前置きの `変数=値`、括弧と `>` を含む引数のあるコマンドが拒否された。`&` で裏で起動し `kill %1` で止める形、heredoc、`--script` のファイルは通った（claude 2.1.316、gl:nas/scripts#23）。シングルクォートの `'#...'` を含むheredocも拒否された（#24）。sandboxの外から起動した `claude -p` のsandboxでは、完全なChromiumは `process_singleton` のunixソケットを作れずに落ちた。`chrome-headless-shell` は撮れた。`Edit(//<絶対パス>/**)` の規則で、画像の置き場所への `Write` のツールだけが通り、ほかへの `Write` は拒否された。`Write(...)` の規則と、`./` で始まる規則では、書き込みは許されなかった（#25）。この形にしたあと、計画の実行（`claude -p`）がダッシュボードを起動し、`Write` で書いたJSで画面を組み替えて、今の画面と2つの案の画像を撮った。拒否された操作は無く、3枚とも選択肢の位置に付いた（2026-11-09、gl:nas/scripts#26）Playwrightのchromium-1243。`SWITCHBOARD_BROWSER=... go test -tags realbrowser ./internal/shot/` で再現できる | 2026-11-09 |
| V59 | `setup_command` のあるプロジェクトで、常駐のエージェントの実行は、worktreeに依存を用意してからclaudeを起動するか。実行の中でテストは動くか | 動く。`gh:uskanda/smarternews` に `setup_command: pnpm install --frozen-lockfile` を足し、issue 107への返答で実行を始めた。準備のコマンドは1.1秒で終わり、空だったworktreeの `node_modules` に1205個のパッケージが入った。監査ログに `setup.ran` が残り、実行のディレクトリに `setup.log` が残った。実行の中の `pnpm exec vitest run` は拒否されずに動き、44件のテストが通った。同じissueの前の実行は、レジストリに接続できずに1件も動かせなかった。2026-11-12、実行 `fc2ce0e6750803ec`、claude 2.1.322 |
| V60 | 非公開のGitHubのリポジトリで、計画の質問に `gh --attach` で付けた画像（`https://github.com/user-attachments/assets/<uuid>`）を、受信箱のトークン（fine-grained PAT）で読めるか。転送の先が `private-user-images.githubusercontent.com` か | 未確認（`add-github-plan-question-images` の実装の時点。公開のリポジトリでは、認証なしで読めた）。非公開のリポジトリの質問をWeb表示で開いて確かめる。読めなければ、Web表示は画像へのリンクを出す | |

### 検証で分かった、設計に関わる点

- 親のClaude Codeのセッションから起動すると、子の `claude -p` は環境変数 `CLAUDE_CODE_SESSION_ID` を引き継ぎ、親と同じセッションIDで動いた。エージェントは環境を許可リストで組み立てる設計（DESIGN.md 4.4節）なので、この変数は渡さない
- `--setting-sources user` ではプロジェクトの `CLAUDE.md` も読み込まれない（V17）。ADR-021はプロジェクト設定を読み込まないと決めているため、リポジトリの指示をプロンプトで渡す方法か、ADR-018の検査を前提にプロジェクト設定を読み込む方法かを、Phase 1の成果物で決める
- `default` の権限モードでは、承認の要る呼び出しでPermissionRequestフックが動き、判定を返さなければ拒否された。`dontAsk` ではこのフックが動かないため、拒否の記録は `permission_denials` から取る
- denyルールは境界にならない（V5）。Claudeのプロセスにフォージの資格情報を渡さないADR-012が、push対策の本体になる
