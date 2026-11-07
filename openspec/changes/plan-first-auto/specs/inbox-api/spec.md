## MODIFIED Requirements

### Requirement: プロジェクトの一覧
APIは、`switchboard inbox serve` が動く端末の担当プロジェクトの一覧を返さなければならない（MUST）。
各項目は、`ref`、`path`、`plan_first`、`plan_first_mode`、クローンの有無、無い状態ラベルの名前を持つ。`plan_first_mode` は設定の `<projects[].plan_first>`（`always`、`auto`、`never`）で、`plan_first` は、古いクライアントのために、それが `always` か `auto` のとき真とする。クローンの有無は、`path` がgitのリポジトリかどうかで決める。状態ラベルは、受信箱のトークンで読む。
一覧は、追加が有効かどうかも返す。有効になるのは、`<inbox.project_root>` があるときである。

#### Scenario: 足したばかりのプロジェクト
- **WHEN** エージェントがまだcloneしていない `gh:uskanda/notes` が設定にある状態で、一覧を求める
- **THEN** `gh:uskanda/notes` は、クローンが無く、6つの状態ラベルが無いものとして返る

#### Scenario: ラベルを読めない
- **WHEN** フォージに届かない状態で、一覧を求める
- **THEN** 各項目のクローンの有無は返り、ラベルは読めなかったことが返る

#### Scenario: plan_firstがautoのプロジェクト
- **WHEN** `plan_first: auto` の `gh:uskanda/notes` が設定にある状態で、一覧を求める
- **THEN** `gh:uskanda/notes` は、`plan_first_mode` が `auto`、`plan_first` が真で返る

### Requirement: プロジェクトの追加の内容
APIは、`ref` と、`plan_first_mode`（`always`、`auto`、`never`）か真偽値の `plan_first` を受けて、設定にプロジェクトを1つ追加しなければならない（MUST）。
`plan_first_mode` があれば、足す項目の `plan_first` はその値とする。無ければ、`plan_first` が真なら `always`、偽か無いなら `never` とする。`plan_first_mode` がそのほかの値なら、APIは追加せず `invalid` の誤りを返さなければならない（MUST）。
足す項目の `path` は `<inbox.project_root>` の下のリポジトリ名、`host` はこの端末の名前とする。`base_branch` は、受信箱のトークンで読んだリポジトリの既定のブランチとする。そのほかの項目は書かない。
書き込みは、設定の「プロジェクトの追加の書き込み」に従う。

#### Scenario: GitHubのプロジェクトを足す
- **WHEN** 端末 `ubuntu-a` の `<inbox.project_root>` が `~/repos` である。既定のブランチが `main` の `gh:uskanda/notes` を、`plan_first_mode` を `auto` にして追加する
- **THEN** 足される項目は、`path: ~/repos/notes`、`host: ubuntu-a`、`base_branch: main`、`plan_first: auto` を持つ

#### Scenario: 古いクライアントが真偽値で足す
- **WHEN** `plan_first_mode` を持たず、`plan_first` が真の要求で `gh:uskanda/notes` を追加する
- **THEN** 足される項目は `plan_first: always` を持つ

#### Scenario: 知らない値
- **WHEN** `plan_first_mode` が `sometimes` の要求で追加する
- **THEN** 設定ファイルは変わらず、`invalid` の誤りが返る

## ADDED Requirements

### Requirement: issueを作れる先の計画のモード
issueを作れる先の一覧の各項目に、APIは、そのプロジェクトの `<projects[].plan_first>` を `plan_first_mode` として返さなければならない（MUST）。
APIは、古いクライアントのために、`plan_first_mode` が `always` か `auto` のとき真の `plan_first` も返さなければならない（MUST）。

#### Scenario: autoのプロジェクト
- **WHEN** `plan_first: auto` の `gh:o/r` を含む設定で、issueを作れる先の一覧を求める
- **THEN** `gh:o/r` の項目は、`plan_first_mode` が `auto`、`plan_first` が真で返る
