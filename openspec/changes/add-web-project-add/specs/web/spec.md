## ADDED Requirements

### Requirement: 設定の画面のプロジェクトの節
設定の画面は、指示の節より上に、プロジェクトの節を表示しなければならない（MUST）。
プロジェクトの節は、この端末の担当プロジェクトごとに、`ref`、`path`、`plan_first`、クローンの有無、無い状態ラベルを表示する。

#### Scenario: 設定の画面を開く
- **WHEN** 担当プロジェクトが2つある端末で、設定の画面を開く
- **THEN** 指示の節の上に、2つのプロジェクトとそれぞれの状態が表示される

### Requirement: クローンが無いときの案内
クローンが無いプロジェクトについて、プロジェクトの節は、エージェントのトークンの対象にそのリポジトリが入っているかを確かめるよう示さなければならない（MUST）。

#### Scenario: cloneできていない
- **WHEN** クローンの無いプロジェクトがある状態で、設定の画面を開く
- **THEN** そのプロジェクトの行に、エージェントのトークンの対象を確かめるよう示す文が表示される

### Requirement: プロジェクトの追加の欄
追加が有効なとき、プロジェクトの節は、`ref` の入力と `plan_first` の切り替えを持つ追加の欄を表示しなければならない（MUST）。`plan_first` の初期値はオンとする。
追加が無効なとき、プロジェクトの節は、追加の欄の代わりに、`<inbox.project_root>` を設定すれば追加できることを表示しなければならない（MUST）。
追加の欄は、`path`、`host`、`base_branch` などほかの項目を入力させてはならない（MUST NOT）。

#### Scenario: 追加が無効
- **WHEN** `<inbox.project_root>` の無い端末で、設定の画面を開く
- **THEN** 追加の欄は表示されず、`<inbox.project_root>` を設定すれば追加できることが表示される

### Requirement: 追加の結果の表示
追加に成功したとき、WUIは、プロジェクトの一覧を取り直し、足したプロジェクトを表示しなければならない（MUST）。応答にwebhookの案内があれば、それも表示する。
追加が拒まれたとき、WUIは、追加の欄に、拒まれた理由を表示しなければならない（MUST）。

#### Scenario: 中継を使う端末で足す
- **WHEN** `<agent.relay>` のある端末で、`gh:uskanda/notes` を追加する
- **THEN** 一覧に `gh:uskanda/notes` が現れ、`switchboard relay hook add gh:uskanda/notes` を実行するよう表示される

#### Scenario: 同じプロジェクトを足す
- **WHEN** 既にある `gh:uskanda/switchboard` を追加する
- **THEN** 追加の欄に、既にあることが表示される

### Requirement: プロジェクトの追加のPOSTの検査
プロジェクトの追加のPOSTについて、`<switchboard-web>` は、本文がJSONであることを確かめなければならない（MUST）。
同じPOSTについて、`<switchboard-web>` は、`Origin` と `Host` が `<listen>` のアドレスと一致することを確かめなければならない（MUST）。一致しないか、`Origin` が無いときは、追加せずに403を返す。

#### Scenario: 外のWebページから送られる
- **WHEN** 別のオリジンのWebページが、WUIのプロジェクトの追加のURLへPOSTを送る
- **THEN** 403が返り、設定ファイルは変わらない
