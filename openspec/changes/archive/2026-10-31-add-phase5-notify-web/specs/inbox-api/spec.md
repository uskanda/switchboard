## Purpose

受信箱の一覧、1件の詳細、返答を、同じ端末の別のプロセスから使えるように、UnixソケットのAPIとして出す。受信箱のトークンを扱うのは、核のこのプロセスだけである。

## ADDED Requirements

### Requirement: 待ち受け
`switchboard inbox serve` は、APIをUnixソケットで待たなければならない（MUST）。TCPのポートで待ってはならない（MUST NOT）。
ソケットの場所は、`--socket` で指定する。指定が無ければ、systemdの `RUNTIME_DIRECTORY` の下の `inbox.sock` とする。`RUNTIME_DIRECTORY` も無ければ、`$XDG_RUNTIME_DIR/switchboard/inbox.sock` とする。
ソケットのファイルは、所有者だけが読み書きできる権限（0600）で作らなければならない（MUST）。
`switchboard inbox serve` は、受信箱のトークン（`inbox_token_ref`）を使い、エージェント用のトークンを使ってはならない（MUST NOT）。

#### Scenario: 起動する
- **WHEN** `switchboard inbox serve --socket <path>` を実行する
- **THEN** `<path>` に0600のソケットができ、TCPのポートは開かない

### Requirement: 一覧
APIは、`switchboard inbox --json` と同じ項目の待ちの一覧を返さなければならない（MUST）。並びと、設定に無いプロジェクトを出さないことも、`switchboard inbox` と同じにする。
一部のフォージに届かなかったときは、取得できた一覧と、届かなかったフォージとその理由を、ともに返す。
APIは、一覧を60秒キャッシュしてよい（MAY）。返答を投稿したら、キャッシュを捨てなければならない（MUST）。

#### Scenario: GitLabに届かない
- **WHEN** GitHubに待ちが1件あり、GitLabのAPIが失敗する状態で、一覧を求める
- **THEN** GitHubの1件と、GitLabに届かなかったことと理由が返る

#### Scenario: 続けて求める
- **WHEN** 一覧を求めてから60秒以内に、もう一度求める
- **THEN** フォージには問い合わせず、前の一覧が返る

### Requirement: 1件の詳細
APIは、`gh:owner/repo#123` か `gl:group/project#123` の形で指定したissueの詳細を返さなければならない（MUST）。詳細は、`switchboard show` と同じ内容（最後の質問の全文と、拒否された操作の一覧）である。これに、タイトル、issueのURL、待ちかどうかを足す。
行番号での指定は受けない。

#### Scenario: 1件を開く
- **WHEN** 待ちのGitLabのissueを `gl:nas/switchboard-sandbox#6` で求める
- **THEN** そのissueの質問の全文、タイトル、URLが返る

### Requirement: 返答
APIは、指定したissueに返答を投稿しなければならない（MUST）。投稿の前の確認は、`switchboard reply` と同じにする。issueを取り直して `agent:waiting` を確かめ、空の返答とマーカーで終わる返答は拒む。
待ちでないときは、投稿せずに、待ちでないことを区別できる誤りを返さなければならない（MUST）。

#### Scenario: 既に再開していた
- **WHEN** `agent:waiting` が外れたissueに返答を送る
- **THEN** 投稿されず、待ちでないことを示す誤りが返る

### Requirement: 公開のクライアント
核のモジュールは、このAPIを呼ぶGoのパッケージ `<inboxapi>` を、`internal/` の外に公開しなければならない（MUST）。パッケージは、要求と応答の型と、待ちでないことを示す誤りを持つ。
`<inboxapi>` は、フォージのトークンを扱ってはならない（MUST NOT）。

#### Scenario: 別のモジュールから呼ぶ
- **WHEN** `web/` のモジュールが `<inboxapi>` で一覧を求める
- **THEN** ソケットを通して `switchboard inbox serve` の一覧が返る
