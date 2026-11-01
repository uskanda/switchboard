## ADDED Requirements

### Requirement: プロジェクトの追加の親ディレクトリ
設定は、`<inbox.project_root>` に、追加したプロジェクトのcloneを置く親ディレクトリを持てなければならない（MUST）。先頭の `~` は、利用者のホームディレクトリとして読む。
`~` を読んだあとの値が絶対パスでないとき、`switchboard` は設定の誤りとして拒まなければならない（MUST）。
`<inbox.project_root>` が無い設定は、今までどおり読めなければならない（MUST）。そのとき、プロジェクトの追加は無効になる。

#### Scenario: 相対パス
- **WHEN** `<inbox.project_root>` が `repos` の設定を読む
- **THEN** 設定の誤りとして、`<inbox.project_root>` が報告される

#### Scenario: キーが無い
- **WHEN** `<inbox.project_root>` の無い設定を読む
- **THEN** 誤りにならず、プロジェクトの追加は無効になる

### Requirement: プロジェクトの追加の書き込み
プロジェクトを追加するとき、`switchboard` は、設定ファイルの `projects` の末尾に1項目を足さなければならない（MUST）。ほかの項目、コメント、キーの並びは変えない。
設定ファイルがシンボリックリンクのとき、`switchboard` はリンク先のファイルを書き換えなければならない（MUST）。リンクは残す。

#### Scenario: コメントのある設定に足す
- **WHEN** コメントを含む設定ファイルに、`gh:uskanda/notes` を追加する
- **THEN** `projects` の末尾に `gh:uskanda/notes` の項目が足され、元のコメントと項目はそのまま残る

#### Scenario: シンボリックリンクの設定
- **WHEN** `config.yaml` が別のファイルへのシンボリックリンクで、プロジェクトを追加する
- **THEN** リンク先のファイルに項目が足され、`config.yaml` はシンボリックリンクのまま残る

### Requirement: 書き込みの前の検証
プロジェクトを足したあとの設定が、設定の検証を通らないとき、`switchboard` は設定ファイルを書き換えてはならない（MUST NOT）。

#### Scenario: 足すと誤りになる
- **WHEN** 手で書いた設定に検証を通らない項目があり、その状態でプロジェクトを追加する
- **THEN** 設定ファイルは変わらず、誤りのあるキーが報告される

### Requirement: 手の編集との衝突
追加のために設定ファイルを読んでから書くまでに、その中身が変わったとき、`switchboard` は設定ファイルを書き換えてはならない（MUST NOT）。誤りとして、設定ファイルが別に変更されたことを返す。

#### Scenario: 追加の途中で手で編集した
- **WHEN** 追加の処理がフォージに実在を問い合わせている間に、利用者が設定ファイルを手で保存する
- **THEN** 項目は足されず、設定ファイルの別の変更を示す誤りが報告される。手で保存した内容は残る

### Requirement: 元の設定の退避
設定ファイルを書き換える前に、`switchboard` は、元の中身を、設定ファイルと同じディレクトリの `<config.yaml.bak>` に保存しなければならない（MUST）。前の退避があれば、上書きする。
書き換えは、一時ファイルに書いてから置き換える形で行わなければならない（MUST）。書き換えの途中で止まっても、設定ファイルは元の中身か新しい中身のどちらかである。

#### Scenario: 追加のあとの退避
- **WHEN** プロジェクトを追加する
- **THEN** `<config.yaml.bak>` に追加の前の中身が残る

### Requirement: 設定の読み直し
`switchboard agent run` と `switchboard inbox serve` は、動いている間、10秒ごとに設定ファイルの更新の時刻と中身を確かめなければならない（MUST）。
中身が変わり、検証を通ったとき、`switchboard` は `projects` だけを新しい設定に入れ替えなければならない（MUST）。再起動は要らない。

#### Scenario: 手でプロジェクトを足した
- **WHEN** `switchboard agent run` が動いている間に、`config.yaml` の `projects` に手で1項目を足して保存する
- **THEN** 10秒ほどのうちに、エージェントはそのプロジェクトを扱い始める

### Requirement: projects以外の変化
読み直した設定で `projects` 以外の節が変わっているとき、`switchboard` はその節を反映してはならない（MUST NOT）。再起動するまで反映しないことを、変わった節の名前とともにログに残す。

#### Scenario: 間隔を変えた
- **WHEN** `switchboard agent run` の動作中に、`<agent.poll_interval>` を変えて保存する
- **THEN** ポーリングの間隔は変わらない。`agent` の変化は再起動まで反映しないと、ログに残る

### Requirement: 読み直しの誤り
読み直した設定が検証を通らないとき、`switchboard` は前の設定のまま動き続けなければならない（MUST）。誤りのあるキーをログに残す。

#### Scenario: 書きかけの設定
- **WHEN** `switchboard agent run` が動いている間に、`projects` の項目から `path` を消して保存する
- **THEN** エージェントは前の設定のまま動き続け、ログには `path` の欠落を残す
