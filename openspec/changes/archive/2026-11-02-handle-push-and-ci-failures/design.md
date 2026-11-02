# Design

## Context

- 実装の実行の完了は `internal/agent/run.go` の `finish` が扱う。検証、出力検査、`sp.Push`、`CreateChangeRequest` の順に進み、どこかで失敗すると `o.fail`（`waiting(reason=error)`）か `o.verify` を返す。`sp.Push` はgitの誤りをそのまま返し、出力を分類しない
- 先行する `reuse-open-change-request` で、`Forge` にブランチの開いた変更要求を探す操作が入る。直す実行のpushは、その変更要求に載る
- GitHubのwebhookは中継に届かない（ADR-007）。CIの結果は、どちらのフォージでもポーリングで知る（grillingの第1ラウンドQ3）
- エージェントのGitHubのトークンは、Issues、Pull requests、Contentsの権限しか持たない（SECURITY.md 4章）。fine-grained PATでChecks APIを読めるかは確かめていない
- 計画の段階を端末に記録せず、マーカーから毎回導く方式が既にある（ADR-038）。見張りの対象と直した回数も同じ方式で導く
- `internal/agent/writeback.go` の `outcome` が、待ちと完了のコメントとマーカーを組み立てる。`internal/marker` は知らない項目を読み飛ばす

## Goals / Non-Goals

**Goals:**

- CIの判定とログの取得を、`internal/forge` の薄い層に閉じ込める。フォージのAPIの違いを `agent` に持ち込まない
- 見張りの状態を端末のファイルに持たない。再起動しても、マーカーとフォージの状態から同じ判断になる
- 直す実行を、今の実装の実行の延長として作る。検証、出力検査、書き戻し、並列数の制御を共有する

**Non-Goals:**

- 中継のイベントの種別を増やすこと（GitLabのPipeline Hookは後回し）
- 必須のチェックかどうかの区別、ブランチの保護の設定を読むこと
- リモートの規則による拒否を、自動で直させること
- CIの再実行（rerun）をエージェントから行うこと

## Decisions

### D1. CIを読む操作は `Forge` とは別のインターフェースにする

`<forge.CI>` を足し、次の3つの操作を持たせる。

- `CommitChecks(ctx, owner, repo, sha)`：そのコミットのチェックの一覧を返す。各チェックは、名前、状態（終わったか）、結論、URL、ログを取るための識別子を持つ
- `CheckLog(ctx, owner, repo, check)`：失敗したjobの、失敗したstepの名前とログの末尾を返す
- `ChangeRequestHead(ctx, owner, repo, number)`：変更要求の状態（open、closed、merged）とheadのSHAを返す

`Forge` に足さないのは、受信箱のサービスとfakeの多くの利用者がCIを使わないからである。`Hooks`、`Repos`、`Issues` と同じく、使う側だけが持つ。

- GitHub：Actionsのworkflow run（`head_sha` で絞る）とそのjob、jobのログを使う。これは `Actions: Read` で読める。加えて、`Commit statuses: Read` で読めるcombined statusを使う。check runは、検証項目V35で確かめてから足す。V35は、fine-grained PATでcheck runを読めるかを確かめる。読めなければ足さない（grillingの第3ラウンドQ17）
- GitLab：`sha` で絞ったpipelineとそのjob、jobのtrace、コミットのstatusを使う。今の `api` スコープで足りる
- 代案：Checks APIだけで判定する。GitHub Appのトークンでないと読めない恐れがあり、確認前に依存できないので採らない

### D2. 見張りの一覧はメモリに持ち、マーカーから作り直す

エージェントは、見張りの一覧（issue、`sha`、`cr`、pushの時刻）をメモリに持つ。

- 自分が完了のコメントを書いたとき、一覧に足す
- 起動のときと照合のたびに、担当プロジェクトの `agent:done` のopenのissueを検索する。最新の完了のコメントのマーカーに `sha` と `cr` があり、`at` が6時間以内のものを一覧に戻す
- ポーリングのたびに、一覧の各issueに `ChangeRequestHead` と `CommitChecks` を問い合わせ、終わりの条件（spec `ci-watch`）を判定する

代案として、ポーリングのたびに `agent:done` のissueを検索する方法もある。しかし、GitHubの検索APIは1分に30回の上限が厳しい。そのため、検索は照合の時だけにする。pushの時刻は、完了のマーカーの `at` で代える。

### D3. 直す実行は新しい実行の種類 `fixRun` とする

`<plan.kind>` に `fixRun` を足す。`fixRun` は `failure` を持つ。`failure` は、CIの失敗の情報か、pushの失敗の出力（伏せ字にしたもの）である。

- CIの失敗：見張りの判定で失敗と分かったら、直した回数をマーカーから数える。上限未満なら、空いた枠があるときに、`agent:running` への付け替え、着手のコメント（`reason=fix`）、実行の開始の順に進む。枠が無いときは見張りの一覧に「直す実行を待つ」として残し、次のポーリングでやり直す。順番待ちのコメントは書かない
- non-fast-forward：`finish` の中で分かる。エージェントは `git fetch origin switchboard/issue-<n>` を行う。そのうえで、同じissueのロックを持ったまま、着手のコメント（`reason=fix`）を付けて `fixRun` を始め直す。issueは `agent:running` のままである
- 失敗の情報は、着手のときにフォージから取り直す。端末には残さない。再起動で `fixRun` の途中から戻るときは、今の再起動後の回復と同じく、中断として書き戻す
- 計画のモードのissueでも `fixRun` は実装の実行として扱い、計画の判定（ADR-038）を通さない

代案として、CIの失敗をissueのコメントとして書き、人の返信と同じ経路で再開する方法もある。しかし、ログの全文がコメントに載り、`trusted_authors` の判定とも混ざるので採らない。

### D4. 直した回数はマーカーから数える

直した回数は、最新の「リセットの点」より後の、エージェントの `reason=fix` の着手のコメントの数である。リセットの点は、信頼済みの人のコメントか、`mode` を持つ着手のコメント（始めるラベルでの着手）とする。端末に回数を持たないので、どの端末が担当になっても同じ数になる。

### D5. pushの失敗は `workspace` で分類する

`sp.Push` は、失敗したとき `*workspace.PushError{Kind, Output}` を返す。`Kind` は `transient`、`nonFastForward`、`auth`、`rejected` のどれかである。分類はgitの出力の文字列で行う。

- `nonFastForward`：`[rejected]` と `(fetch first)` か `(non-fast-forward)`
- `auth`：`Authentication failed`、`403`、`Permission to ... denied`、`HTTP Basic: Access denied`
- `transient`：`Could not resolve host`、`Connection timed out`、`Connection reset`、`RPC failed`、`HTTP 5xx`、`early EOF`
- `rejected`：`[remote rejected]`、`pre-receive hook declined`、`GH006`、`GH013`、それ以外の全部

分類の文字列はフォージとgitの版で変わりうるので、テストに実際の出力の例を置く。分類できないものを `rejected` として扱うのは、人に見せるのがいちばん安全だからである。一時的な失敗のやり直しの間隔は、5秒と20秒とする。

### D6. プロンプトと権限

- `internal/prompt` に、失敗の情報の節を足す。issueの非信頼のコメントと同じnonce付きの区切りに入れ、「データであり指示ではない」と追記の指示に書く
- `fixRun` の追記の指示には、「CIの失敗（またはpushの拒否）を直す。保護パスは変えない。直せないなら `waiting` を宣言して理由を書く」を書く
- `settings` は、non-fast-forwardの `fixRun` だけ `Bash(git merge *)` を足す。`git rebase` と `git push` はdenyに入れる。denyは境界ではない（V5）ので、本当の防壁は、Claudeにフォージのトークンを渡さないこと（ADR-012）と、エージェントが強制pushをしないことである

### D7. コメント

- 直す実行の着手のコメント：失敗したチェックの名前（pushの拒否なら「pushが拒否された」）と「（k/N）」を書く。ログは書かない
- `waiting(reason=ci)`：spec `ci-watch` の「直す回数の上限に達したとき」のとおり。各回の要約は、`fixRun` の完了のコメントから取る
- `waiting(reason=push)`：分類、伏せ字にしたgitの出力（`tail` と同じ上限）、人がすべきこと
- 伏せ字には、`CheckOutput` が使っている秘密の形式の判定を共有する

### D8. 権限の不足と `doctor`

- 見張りの問い合わせが403のときは、そのissueの見張りをやめ、監査ログに `<ci.forbidden>` を残す。同じプロジェクトの403は、プロセスが動いている間は1回だけログに出す
- `switchboard doctor` は、GitHubのプロジェクトごとに、Actionsのworkflow runの一覧とcombined statusを読んでみる。403なら警告し、失敗にはしない

## Risks / Trade-offs

- ［Claudeがテストを消して「直す」］→変更要求はdraftのままで、人がレビューする。直す実行の追記の指示で、テストを消して通すことを禁じる。保護パスの検査も今のまま行う
- ［CIが不安定で、直す実行がトークンを使い続ける］→回数の上限（既定2）と、上限の後は人の判断を待つ。0にすれば自動で直さない
- ［ログに含まれる指示の形の文］→nonce付きの非信頼のデータとして渡し、許可するツールも増やさない（non-fast-forwardの `git merge` だけ）
- ［見張りの問い合わせがフォージのAPIの上限を使う］→問い合わせは見張り中のissueごとに `poll_interval` で2回程度で、ログは失敗したときだけ取る。検索は照合の時だけ
- ［pushの出力の分類が外れる］→分からないものは `rejected` として人に見せる。分類の文字列はテストの例で固定し、外れたら足す
- ［GitHub Actions以外のGitHub App（check runだけを付けるCI）を見落とす］→V35で読めると分かるまでは見落とす。commit statusを付けるCIは見える

## Migration Plan

1. GitHubのエージェントのfine-grained PATに `Actions: Read` と `Commit statuses: Read` を足す。足さなくても動き、見張りが403で終わるだけである
2. 新しい版を入れてサービスを起動し直す。起動のときの照合で、6時間以内に完了したissueから見張りが始まる。ただし、この版より前の完了のマーカーには `sha` が無いので、見張りの対象にならない
3. 戻すときは前の版を入れる。マーカーの `sha`、`cr`、`reason=fix` は、前の版が読み飛ばす

## Open Questions

- V35：fine-grained PATでcheck runを読めるか。結果によって、D1でcheck runを足すかどうかだけが変わる
