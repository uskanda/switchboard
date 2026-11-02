# inbox-api Specification

## Purpose

受信箱の一覧、1件の詳細、返答を、同じ端末の別のプロセスから使えるように、UnixソケットのAPIとして出す。受信箱のトークンを扱うのは、核のこのプロセスだけである。

## Requirements

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
APIは、1件の詳細に、issueの本文と、コメントの一覧を古い順に含めなければならない（MUST）。コメントの各要素は、次の項目を持つ。

- 書いた人
- 投稿の時刻
- 本文
- エージェントの投稿かどうか
- エージェントの投稿の種類（着手、進捗、待ち、完了など）

APIは、本文とコメントから、エージェントの投稿のマーカーを取り除いて返さなければならない（MUST）。
行番号での指定は受けない。
APIは、エージェントのアカウントのIDを、プロセスが動いている間、1度引いたら使い回さなければならない（MUST）。引けなかったときは覚えず、次に求められたときに引き直す。
フォージのアダプタは、issue、コメント、編集した人の3つの問い合わせを、順に待たずに同時に送らなければならない（MUST）。

#### Scenario: 1件を開く
- **WHEN** 待ちのGitLabのissueを `gl:nas/switchboard-sandbox#6` で求める
- **THEN** そのissueの質問の全文、タイトル、URL、本文、コメントの一覧が返る

#### Scenario: エージェントの投稿を含む
- **WHEN** 着手のコメントと人のコメントがあるissueの詳細を求める
- **THEN** 着手のコメントはエージェントの投稿で種類が着手として返り、どちらの本文にもマーカーは含まれない

### Requirement: 返答
APIは、指定したissueに返答を投稿しなければならない（MUST）。投稿の前の確認は、`switchboard reply` と同じにする。issueを取り直して、openで `agent:waiting` か `agent:done` が付いていることを確かめ、空の返答とマーカーで終わる返答は拒む。
どちらでもないときは、投稿せずに、待ちでないことを区別できる誤りを返さなければならない（MUST）。

#### Scenario: 既に再開していた
- **WHEN** `agent:waiting` が外れて `agent:running` が付いたissueへ、返答を送る
- **THEN** 投稿されず、待ちでないことを示す誤りが返る

#### Scenario: 完了のissueに返答する
- **WHEN** openのまま `agent:done` のissueに返答を送る
- **THEN** 返答が投稿される

### Requirement: 公開のクライアント
核のモジュールは、このAPIを呼ぶGoのパッケージ `<inboxapi>` を、`internal/` の外に公開しなければならない（MUST）。パッケージは、要求と応答の型と、待ちでないことを示す誤りを持つ。
`<inboxapi>` は、フォージのトークンを扱ってはならない（MUST NOT）。

#### Scenario: 別のモジュールから呼ぶ
- **WHEN** `web/` のモジュールが `<inboxapi>` で一覧を求める
- **THEN** ソケットを通して `switchboard inbox serve` の一覧が返る

### Requirement: 追加の指示の読み出し
APIは、全体の指示と、設定にあるプロジェクトごとの指示を返さなければならない（MUST）。
各項目は、対象（全体か、`gh:owner/repo` の形のプロジェクト）、今の内容、`switchboard inbox serve` が動く端末の名前を持つ。ファイルが無い指示は、空の内容として返す。

#### Scenario: プロジェクトの指示だけがある
- **WHEN** 全体の指示が無く、`gh:uskanda/switchboard` の指示だけがある状態で、指示を求める
- **THEN** 全体の指示は空で、`gh:uskanda/switchboard` の指示は今の内容で返り、ほかのプロジェクトは空で返る

### Requirement: 追加の指示の保存
APIは、指定した対象の指示を保存しなければならない（MUST）。保存の前の検査と記録は、`plan-instructions` の保存と同じにする。
検査で拒んだとき、APIは保存せずに、大きさの上限を超えたことか、秘密らしいものがあることを区別できる誤りを返さなければならない（MUST）。
設定に無いプロジェクトを指定したとき、APIは保存せずに誤りを返さなければならない（MUST）。

#### Scenario: 大きすぎる指示
- **WHEN** 40 KiBの指示を保存しようとする
- **THEN** 保存されず、大きさの上限を超えたことを示す誤りが返る

#### Scenario: 設定に無いプロジェクト
- **WHEN** 設定に無い `gh:someone/other` の指示を保存しようとする
- **THEN** 保存されず、誤りが返る

### Requirement: 一覧の要素の更新の手がかり
APIは、一覧の各要素に、issueの更新の時刻と、人が書いたコメントの数を含めなければならない（MUST）。人が書いたコメントの数は、GitLabでは `user_notes_count` で、GitHubでは0とする。

#### Scenario: GitLabのissue
- **WHEN** `user_notes_count` が3の待ちのGitLabのissueを一覧に出す
- **THEN** その要素の人のコメントの数は3である

### Requirement: 変化の知らせ
APIは、一覧の変化をSSEで知らせる口を持たなければならない（MUST）。
口につないだとき、APIは、そのときの一覧を最初の知らせとして送らなければならない（MUST）。
口につながっているクライアントがある間、APIは、`<inbox.refresh_interval>` ごとにフォージから一覧を取り直さなければならない（MUST）。
取り直した一覧が前の一覧と違うとき、APIは、新しい一覧を、つながっているすべてのクライアントに送らなければならない（MUST）。
口につながっているクライアントが無い間、APIは、フォージへの間隔ごとの問い合わせをしてはならない（MUST NOT）。
返答を投稿したとき、APIは、間隔を待たずに一覧を取り直して知らせなければならない（MUST）。
APIは、知らせの一覧を、GETの一覧のキャッシュにも使わなければならない（MUST）。

#### Scenario: 待ちが増える
- **WHEN** クライアントが口につながっている間に、別のissueが待ちになる
- **THEN** 次の間隔の取り直しで、そのissueを含む一覧がクライアントに届く

#### Scenario: 誰もつながっていない
- **WHEN** 口につながっているクライアントが無いまま、間隔の3倍の時間がたつ
- **THEN** APIはフォージに問い合わせない

#### Scenario: 一覧が変わらない
- **WHEN** 取り直した一覧が前の一覧と同じである
- **THEN** クライアントには何も届かない

### Requirement: プロジェクトの一覧
APIは、`switchboard inbox serve` が動く端末の担当プロジェクトの一覧を返さなければならない（MUST）。
各項目は、`ref`、`path`、`plan_first`、クローンの有無、無い状態ラベルの名前を持つ。クローンの有無は、`path` がgitのリポジトリかどうかで決める。状態ラベルは、受信箱のトークンで読む。
一覧は、追加が有効かどうかも返す。有効になるのは、`<inbox.project_root>` があるときである。

#### Scenario: 足したばかりのプロジェクト
- **WHEN** エージェントがまだcloneしていない `gh:uskanda/notes` が設定にある状態で、一覧を求める
- **THEN** `gh:uskanda/notes` は、クローンが無く、6つの状態ラベルが無いものとして返る

#### Scenario: ラベルを読めない
- **WHEN** フォージに届かない状態で、一覧を求める
- **THEN** 各項目のクローンの有無は返り、ラベルは読めなかったことが返る

### Requirement: プロジェクトの追加の内容
APIは、`ref` と `plan_first` を受けて、設定にプロジェクトを1つ追加しなければならない（MUST）。
足す項目の `path` は `<inbox.project_root>` の下のリポジトリ名、`host` はこの端末の名前とする。`base_branch` は、受信箱のトークンで読んだリポジトリの既定のブランチとする。そのほかの項目は書かない。
書き込みは、設定の「プロジェクトの追加の書き込み」に従う。

#### Scenario: GitHubのプロジェクトを足す
- **WHEN** 端末 `ubuntu-a` の `<inbox.project_root>` が `~/repos` である。既定のブランチが `main` の `gh:uskanda/notes` を、`plan_first` を真にして追加する
- **THEN** 足される項目は、`path: ~/repos/notes`、`host: ubuntu-a`、`base_branch: main`、`plan_first: true` を持つ

### Requirement: プロジェクトの追加の前の確認
次のどれかに当たるとき、APIはプロジェクトを追加してはならず、それぞれを区別できる誤りを返さなければならない（MUST NOT）。
- `<inbox.project_root>` が無い
- `ref` が `gh:owner/repo` か `gl:group/project` の形でない
- 同じ `ref` が既に設定にある
- この端末の担当プロジェクトのどれも、そのフォージを使っていない
- 受信箱のトークンで、そのリポジトリを読めない
- `path` にするディレクトリを、別の設定のプロジェクトが既に使っている

#### Scenario: この端末で使っていないフォージ
- **WHEN** この端末の担当プロジェクトがすべて `gh:` の状態で、`gl:nas/tools` を追加する
- **THEN** 追加されず、そのフォージがこの端末で使われていないことと、手で足して `switchboard service install` を実行し直す必要があることを示す誤りが返る

#### Scenario: 存在しないリポジトリ
- **WHEN** フォージに無い `gh:uskanda/no-such-repo` を追加する
- **THEN** 追加されず、リポジトリを読めないことを示す誤りが返る

### Requirement: webhookの案内
`<agent.relay>` のある端末でGitLabのプロジェクトを追加したとき、追加の応答は、`switchboard relay hook add <ref>` の実行を促す文字列を含まなければならない（MUST）。
GitHubのプロジェクトの応答は、この文字列を含んではならない（MUST NOT）。中継の受け口にGitHubは届かないため（ADR-007）、GitHubのプロジェクトはポーリングで扱う。
APIは、webhookを登録してはならない（MUST NOT）。

#### Scenario: 中継を使う端末でGitLabのプロジェクトを足す
- **WHEN** `<agent.relay>` のある端末で、`gl:nas/tools` を追加する
- **THEN** 応答に `switchboard relay hook add gl:nas/tools` が含まれ、フォージにwebhookは作られない

#### Scenario: 中継を使う端末でGitHubのプロジェクトを足す
- **WHEN** `<agent.relay>` のある端末で、`gh:uskanda/notes` を追加する
- **THEN** 応答にwebhookの案内は含まれない

### Requirement: 追加の反映
プロジェクトを追加したとき、APIは、再起動せずに、受信箱の一覧、追加の指示、プロジェクトの一覧の対象にそのプロジェクトを含めなければならない（MUST）。

#### Scenario: 追加の直後に指示を開く
- **WHEN** `gh:uskanda/notes` を追加した直後に、追加の指示を求める
- **THEN** `gh:uskanda/notes` の指示が、空の内容で返る

### Requirement: 順番待ちの詳細
順番待ちのissueの詳細を求められたとき、APIは、状態を順番待ちとして、順番待ちの開始時刻と当たった上限を返さなければならない（MUST）。
順番待ちのissueに `agent:waiting` が付いているとき、APIは、待ちであることと、エージェントの最後の待ちの投稿を返さなければならない（MUST）。返答も、今までどおり受け付ける。
`agent:waiting` が付いていない順番待ちのissueについて、APIは、順番待ちのコメントを最後の投稿として返さなければならない（MUST）。

#### Scenario: 始めるラベルで順番待ちのissueを開く
- **WHEN** エージェントの投稿が順番待ちのコメントだけの、`agent:ready` のissueの詳細を求める
- **THEN** 状態は順番待ちで、最後の投稿は順番待ちのコメントであり、待ちではないと返る

#### Scenario: 返答の待ちで順番待ちのissueに返答する
- **WHEN** `agent:waiting` が付いた順番待ちのissueに、返答を送る
- **THEN** 返答は投稿され、順番が来た実行のプロンプトに入る

### Requirement: 完了の項目
一覧の要素は、openのまま `agent:done` のissueについて、状態 `done` と行番号を持たなければならない（MUST）。完了の時刻を `done_since` に、doneの投稿が名指すPR/MRのURLを `change_url` に入れる。
1件の詳細は、openのまま `agent:done` のissueで `done` を真にしなければならない（MUST）。closeされたissueでは偽にする。

#### Scenario: 完了のissueの要素
- **WHEN** PRを作ってdoneになったopenのissueがある状態で、一覧を求める
- **THEN** そのissueの要素は `state` が `done` で、`done_since` と `change_url` を持つ
