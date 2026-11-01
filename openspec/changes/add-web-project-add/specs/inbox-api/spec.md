## ADDED Requirements

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
`<agent.relay>` のある端末では、追加の応答は、`switchboard relay hook add <ref>` の実行を促す文字列を含まなければならない（MUST）。
APIは、webhookを登録してはならない（MUST NOT）。

#### Scenario: 中継を使う端末
- **WHEN** `<agent.relay>` のある端末で、`gh:uskanda/notes` を追加する
- **THEN** 応答に `switchboard relay hook add gh:uskanda/notes` が含まれ、フォージにwebhookは作られない

### Requirement: 追加の反映
プロジェクトを追加したとき、APIは、再起動せずに、受信箱の一覧、追加の指示、プロジェクトの一覧の対象にそのプロジェクトを含めなければならない（MUST）。

#### Scenario: 追加の直後に指示を開く
- **WHEN** `gh:uskanda/notes` を追加した直後に、追加の指示を求める
- **THEN** `gh:uskanda/notes` の指示が、空の内容で返る
