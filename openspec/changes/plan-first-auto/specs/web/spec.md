## MODIFIED Requirements

### Requirement: プロジェクトの追加の欄
追加が有効なとき、プロジェクトの節は、`ref` の入力と、`agent:ready` の始め方を選ぶ欄を持つ追加の欄を表示しなければならない（MUST）。始め方は、「必要なら計画から始める」（`auto`）、「必ず計画から始める」（`always`）、「すぐに実装する」（`never`）の3つから1つを選ばせ、初期値は `auto` とする。
追加するとき、WUIは、選んだ値を `plan_first_mode` として、`never` 以外なら真の `plan_first` とともに送らなければならない（MUST）。
追加が無効なとき、プロジェクトの節は、追加の欄の代わりに、`<inbox.project_root>` を設定すれば追加できることを表示しなければならない（MUST）。
追加の欄は、`path`、`host`、`base_branch` などほかの項目を入力させてはならない（MUST NOT）。

#### Scenario: 追加が無効
- **WHEN** `<inbox.project_root>` の無い端末で、設定の画面を開く
- **THEN** 追加の欄は表示されず、`<inbox.project_root>` を設定すれば追加できることが表示される

#### Scenario: 追加の欄の初期値
- **WHEN** 追加が有効な端末で、設定の画面を開く
- **THEN** 始め方の3つの選択肢が表示され、「必要なら計画から始める」が選ばれている

#### Scenario: すぐに実装するプロジェクトを足す
- **WHEN** 「すぐに実装する」を選んで `gh:o/notes` を追加する
- **THEN** `plan_first_mode` が `never`、`plan_first` が偽の要求が送られる

## ADDED Requirements

### Requirement: プロジェクトの始め方の表示
プロジェクトの節は、各プロジェクトの `agent:ready` の始め方を、`plan_first_mode` に従って「必要なら計画から始める」「必ず計画から始める」「すぐに実装する」のどれかで表示しなければならない（MUST）。
`plan_first_mode` を返さない古い受信箱のサービスでは、プロジェクトの節は、`plan_first` が真なら「計画から始める」、偽なら「すぐに実装する」と表示しなければならない（MUST）。

#### Scenario: autoのプロジェクト
- **WHEN** `plan_first_mode` が `auto` のプロジェクトがある状態で、設定の画面を開く
- **THEN** そのプロジェクトの行に「必要なら計画から始める」と表示される

### Requirement: 新規issueのreadyの始め方の補足
新規issueのダイアログで選んだプロジェクトの `plan_first_mode` が返っているとき、ダイアログは、`ready` の選択肢に、`always` なら「必ず計画から始めます」、`auto` なら「必要なら計画から始めます」、`never` なら「すぐに実装します」を補わなければならない（MUST）。
`plan_first_mode` が無く `plan_first` だけが返っているとき、ダイアログは、今までどおり真偽値で補わなければならない（MUST）。

#### Scenario: autoのプロジェクトを選ぶ
- **WHEN** `plan_first_mode` が `auto` のプロジェクトを選ぶ
- **THEN** `ready` の選択肢に「このプロジェクトは必要なら計画から始めます」が表示される
