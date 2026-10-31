## Purpose

フォージのwebhookを受け、検証したイベントのIDだけを端末のエージェントへ配る。中継は、issueの本文、コメント、タイトル、ユーザー名、フォージのトークンを持たない。

## ADDED Requirements

### Requirement: webhookの受け口
`switchboard relay serve` は、webhookの受け口を、設定の `<relay.webhook_listen>` のアドレスで待たなければならない（MUST）。既定は `127.0.0.1:<port>` とする。
受け口のパスは、GitHubでは `/webhook/github/<hook-key>`、GitLabでは `/webhook/gitlab/<hook-key>` とする。`hook-key` は、設定でプロジェクトと対応づけた、推測できない値とする。
中継は、知らない `hook-key`、上限（5 MiB）を超えるボディ、検証できない署名のリクエストを破棄しなければならない（MUST）。破棄した理由はログに残す。

#### Scenario: 知らないhook-key
- **WHEN** 設定に無い `hook-key` のパスに、webhookが届く
- **THEN** 中継は404を返し、イベントを配らない

#### Scenario: 大きすぎるボディ
- **WHEN** 5 MiBを超えるボディのwebhookが届く
- **THEN** 中継は署名を検証する前に破棄し、イベントを配らない

### Requirement: GitHubの署名の検証
中継は、GitHubのwebhookの `X-Hub-Signature-256` を、そのプロジェクトのシークレットによるHMAC-SHA256と、定数時間で比較しなければならない（MUST）。一致しないものは破棄する。

#### Scenario: 正しい署名
- **WHEN** そのプロジェクトのシークレットで署名された `issue_comment` のwebhookが届く
- **THEN** 中継はイベントを1件配る

#### Scenario: 署名が違う
- **WHEN** 別のシークレットで署名されたwebhookが届く
- **THEN** 中継は401を返し、イベントを配らない

### Requirement: GitLabの署名の検証
中継は、GitLabのwebhookを、署名トークン（`webhook-id`、`webhook-timestamp`、`webhook-signature`）で検証しなければならない（MUST）。`webhook-timestamp` が中継の時刻から5分を超えてずれたものは破棄する。
設定でそのプロジェクトに `<gitlab_token>: true` が指定されているときに限り、中継は署名トークンの代わりに `X-Gitlab-Token` を使ってもよい（MAY）。その場合は、シークレットと定数時間で比較する。指定が無いプロジェクトで `X-Gitlab-Token` だけのwebhookは破棄する。

#### Scenario: 署名トークン
- **WHEN** そのプロジェクトの署名トークンで署名されたNote Hookが、正しいタイムスタンプで届く
- **THEN** 中継はイベントを1件配る

#### Scenario: 古いタイムスタンプ
- **WHEN** 正しく署名されているが、`webhook-timestamp` が10分前のwebhookが届く
- **THEN** 中継は破棄し、イベントを配らない

#### Scenario: X-Gitlab-Tokenだけが付いている
- **WHEN** `<gitlab_token>` の指定が無いプロジェクトに、正しい `X-Gitlab-Token` だけが付いたwebhookが届く
- **THEN** 中継は破棄し、イベントを配らない

### Requirement: 重複の除去
中継は、配信IDで重複を除かなければならない（MUST）。配信IDは、GitHubでは `X-GitHub-Delivery`、GitLabでは `webhook-id`（`Idempotency-Key`）とする。同じ配信IDのwebhookが72時間以内にもう一度届いたら、イベントを配らずに成功を返す。
配信IDの記録はメモリだけに持つ。

#### Scenario: 再送
- **WHEN** 同じ配信IDのwebhookが2回届く
- **THEN** 中継はイベントを1件だけ配り、2回とも成功を返す

### Requirement: IDだけを配る
中継は、検証したwebhookから、フォージ種別、プロジェクト、issue番号、イベントの種別、配信IDだけを取り出して配らなければならない（MUST）。残りのペイロードは破棄する。
中継は、issueの本文、コメント、タイトル、ユーザー名を、保持や転送をしてはならない（MUST NOT）。ログに残してもならない（MUST NOT）。
GitHubの対象は、`issues`（labeled、unlabeled、closed、reopened、edited）と `issue_comment`（created、edited）である。GitLabでは、Issue HookとNote Hook（issueへのもの）を対象にする。それ以外のイベントは、配らずに成功を返す。

#### Scenario: 本文を含むwebhook
- **WHEN** コメントの本文とユーザー名を含む `issue_comment` のwebhookが届く
- **THEN** 配られるイベントは `forge`、`project`、`issue`、`kind`、`delivery` だけを持ち、本文とユーザー名は、イベントとログのどちらにも出ない

#### Scenario: 対象外のイベント
- **WHEN** GitLabのMerge Request Hookが届く
- **THEN** 中継は成功を返し、イベントを配らない

### Requirement: SSEの配信口
中継は、SSEの配信口を、webhookの受け口とは別のリスナーとして待たなければならない（MUST）。アドレスは設定の `<relay.sse_listen>` で、既定は `127.0.0.1:<別のport>` とする。
配信口は、端末ごとのbearerトークンで認証しなければならない（MUST）。中継は、トークンのハッシュだけを持つ。
中継は、イベントを、つながっているすべての端末に配らなければならない（MUST）。担当の判定はしない。
中継は、直近1,000件のイベントをメモリに持ち、`Last-Event-ID` を付けて再接続した端末に、その後のイベントを配らなければならない（MUST）。

#### Scenario: トークンが無い
- **WHEN** bearerトークンを付けずに配信口へつなぐ
- **THEN** 中継は401を返す

#### Scenario: 再接続
- **WHEN** 端末が切断し、その間に2件のイベントが届き、端末が `Last-Event-ID` を付けて再接続する
- **THEN** 端末は、その2件を受け取る

### Requirement: webhookの登録
`switchboard relay hook add <project>` は、`hook-key` とシークレットを作り、シークレットをSecret Serviceに置かなければならない（MUST）。そのうえで、受信箱のトークンで、そのプロジェクトにwebhookをAPIで登録する。最後に、設定の `<relay>` 節に足す項目を表示する。
登録が403か404で失敗したとき、`relay hook add` は、画面から登録する手順を表示しなければならない（MUST）。手順には、受け口のURL、対象のイベント、シークレットの貼り方を含める。GitLabでは、ローカルへの送信の許可リストに足すアドレスも示す。
シークレットの値は、画面での登録の手順で1回だけ表示してよい（MAY）。それ以外で表示してはならない（MUST NOT）。
`relay hook remove <project>` は、登録したwebhookとシークレットを消さなければならない（MUST）。

#### Scenario: 権限のあるトークン
- **WHEN** webhookを登録できる受信箱のトークンで、`relay hook add gl:nas/switchboard-sandbox` を実行する
- **THEN** webhookが登録され、シークレットはSecret Serviceに置かれ、値は表示されない

#### Scenario: 権限の無いトークン
- **WHEN** webhookを登録する権限の無い受信箱のトークンで、`relay hook add gh:o/r` を実行し、APIが404を返す
- **THEN** シークレットはSecret Serviceに置かれ、画面から登録する手順が表示される

### Requirement: 端末のトークン
`switchboard relay token add <host>` は、端末のbearerトークンを作り、その値をSecret Serviceに置かなければならない（MUST）。そのうえで、設定の `<relay>` 節に足す項目（端末名とトークンのハッシュ）を表示する。
トークンの値を、設定ファイル、ログ、コマンドの引数に出してはならない（MUST NOT）。

#### Scenario: 発行する
- **WHEN** `relay token add usknas` を実行する
- **THEN** トークンの値はSecret Serviceに置かれ、設定に足す項目としてハッシュだけが表示される
