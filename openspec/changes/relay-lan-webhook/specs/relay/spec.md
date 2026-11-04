# Spec Delta

## MODIFIED Requirements

### Requirement: webhookの受け口
`switchboard relay serve` は、webhookの受け口を、設定の `<relay.webhook_listen>` のアドレスで待たなければならない（MUST）。既定は `127.0.0.1:<port>` とする。
`<relay.webhook_listen>` には、ループバックのアドレスか、LANの別の機械のGitLabから受けるためのこの端末のLANのIPアドレスを書ける。中継は、すべてのインターフェース（`0.0.0.0`、`::`）とホスト名で待ってはならない（MUST NOT）。
受け口のパスは、GitHubでは `/webhook/github/<hook-key>`、GitLabでは `/webhook/gitlab/<hook-key>` とする。`hook-key` は、設定でプロジェクトと対応づけた、推測できない値とする。
中継は、知らない `hook-key`、上限（5 MiB）を超えるボディ、検証できない署名のリクエストを破棄しなければならない（MUST）。破棄した理由はログに残す。

#### Scenario: 知らないhook-key
- **WHEN** 設定に無い `hook-key` のパスに、webhookが届く
- **THEN** 中継は404を返し、イベントを配らない

#### Scenario: 大きすぎるボディ
- **WHEN** 5 MiBを超えるボディのwebhookが届く
- **THEN** 中継は署名を検証する前に破棄し、イベントを配らない

#### Scenario: LANのアドレスで待つ
- **WHEN** `<relay.webhook_listen>` が `192.168.0.82:8781` で、`<relay.webhook_url>` を書いていない
- **THEN** 中継は `192.168.0.82:8781` で待ち、フォージに登録するURLの頭は `http://192.168.0.82:8781` になる

#### Scenario: すべてのインターフェース
- **WHEN** `<relay.webhook_listen>` が `0.0.0.0:8781` か `[::]:8781` である
- **THEN** 設定の検証は誤りを報告し、`relay serve` は起動しない

### Requirement: GitLabの署名の検証
中継は、GitLabのwebhookを、署名トークン（`webhook-id`、`webhook-timestamp`、`webhook-signature`）で検証しなければならない（MUST）。`webhook-timestamp` が中継の時刻から5分を超えてずれたものは破棄する。
設定でそのプロジェクトに `<gitlab_token>: true` が指定されているときに限り、中継は署名トークンの代わりに `X-Gitlab-Token` を使ってもよい（MAY）。その場合は、シークレットと定数時間で比較する。指定が無いプロジェクトで `X-Gitlab-Token` だけのwebhookは破棄する。
`<relay.webhook_listen>` がループバックでない間、中継は `X-Gitlab-Token` による検証を受け付けてはならない（MUST NOT）。平文のシークレットがLANを流れるためである。

#### Scenario: 署名トークン
- **WHEN** そのプロジェクトの署名トークンで署名されたNote Hookが、正しいタイムスタンプで届く
- **THEN** 中継はイベントを1件配る

#### Scenario: 古いタイムスタンプ
- **WHEN** 正しく署名されているが、`webhook-timestamp` が10分前のwebhookが届く
- **THEN** 中継は破棄し、イベントを配らない

#### Scenario: X-Gitlab-Tokenだけが付いている
- **WHEN** `<gitlab_token>` の指定が無いプロジェクトに、正しい `X-Gitlab-Token` だけが付いたwebhookが届く
- **THEN** 中継は破棄し、イベントを配らない

#### Scenario: LANの受け口でのgitlab_token
- **WHEN** `<relay.webhook_listen>` が `192.168.0.82:8781` で、`<gitlab_token>: true` の項目がある
- **THEN** 設定の検証は誤りを報告し、`relay serve` は起動しない

#### Scenario: ループバックの受け口でのgitlab_token
- **WHEN** `<relay.webhook_listen>` が `127.0.0.1:8781` か `[::1]:8781` で、`<gitlab_token>: true` の項目がある
- **THEN** 設定は通り、そのプロジェクトでは `X-Gitlab-Token` による検証を使う

### Requirement: webhookの登録
`switchboard relay hook add <project>` は、`hook-key` とシークレットを作り、シークレットをSecret Serviceに置かなければならない（MUST）。そのうえで、受信箱のトークンで、そのプロジェクトにwebhookをAPIで登録する。最後に、設定の `<relay>` 節に足す項目を表示する。
登録が403か404で失敗したとき、`relay hook add` は、画面から登録する手順を表示しなければならない（MUST）。手順には、受け口のURL、対象のイベント、シークレットの貼り方を含める。GitLabでは、ローカルへの送信の許可リストに足すアドレスも示す。
シークレットの値は、画面での登録の手順で1回だけ表示してよい（MAY）。それ以外で表示してはならない（MUST NOT）。
`<relay.webhook_listen>` がループバックでない間、`relay hook add --gitlab-token` は、シークレットを作らずに断らなければならない（MUST）。
`relay hook remove <project>` は、登録したwebhookとシークレットを消さなければならない（MUST）。

#### Scenario: 権限のあるトークン
- **WHEN** webhookを登録できる受信箱のトークンで、`relay hook add gl:nas/switchboard-sandbox` を実行する
- **THEN** webhookが登録され、シークレットはSecret Serviceに置かれ、値は表示されない

#### Scenario: 権限の無いトークン
- **WHEN** webhookを登録する権限の無い受信箱のトークンで、`relay hook add gh:o/r` を実行し、APIが404を返す
- **THEN** シークレットはSecret Serviceに置かれ、画面から登録する手順が表示される

#### Scenario: LANの受け口での--gitlab-token
- **WHEN** `<relay.webhook_listen>` が `192.168.0.82:8781` の設定で、`relay hook add --gitlab-token gl:nas/switchboard-sandbox` を実行する
- **THEN** コマンドは署名トークンを使うよう示して失敗し、Secret Serviceにもフォージにも何も書かない
