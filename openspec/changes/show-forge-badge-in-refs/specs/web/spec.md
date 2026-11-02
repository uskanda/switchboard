## ADDED Requirements

### Requirement: 参照とフォージのバッジ
WUIは、issueとプロジェクトの参照を、`gh:`と`gl:`のプレフィックスを外した形で表示しなければならない（MUST）。
WUIは、参照のすぐ後ろに、フォージのバッジを表示しなければならない（MUST）。バッジの文字は、`gh:`なら「GitHub」、`gl:`なら「GitLab」とする。
バッジの色はフォージごとに分ける。色は補助とし、フォージは文字で読み取れなければならない（MUST）。
この形は、一覧の行、1件の表示の見出し、プロジェクトの節のカード、新規issueの作成のトースト、プロジェクトの追加のトースト、指示の欄の名前に使う。
どちらのプレフィックスも無い参照は、そのまま表示し、バッジを付けてはならない（MUST NOT）。

#### Scenario: 一覧の行
- **WHEN** 一覧に`gh:o/r#12`のissueがある
- **THEN** 行には`o/r#12`と「GitHub」のバッジが並び、`gh:o/r#12`の文字は出ない

#### Scenario: GitLabのissue
- **WHEN** `gl:nas/sb#3`のissueを開く
- **THEN** 見出しには`nas/sb#3`と「GitLab」のバッジが並ぶ

### Requirement: 参照の形を変えない箇所
参照の表示を変えたあとも、URL、APIに送る`ref`、プロジェクトの追加の欄の入力の形（`gh:owner/repo`）は、変えてはならない（MUST NOT）。

#### Scenario: 1件の表示へのリンク
- **WHEN** 一覧で`gh:o/r#12`の行のリンク先を見る
- **THEN** リンク先は`/issue/gh%3Ao%2Fr%2312`のままである

### Requirement: 新規issueのプロジェクトの選択肢
新規issueのダイアログのプロジェクトの選択肢は、プレフィックスを外した参照とフォージの名前を、「o/r GitHub（担当：desk）」の形の文字で表示しなければならない（MUST）。

#### Scenario: 選択肢を見る
- **WHEN** 端末`desk`の担当する`gh:o/r`がある端末で、新規issueのダイアログを開く
- **THEN** プロジェクトの選択肢に「o/r GitHub（担当：desk）」が出る

## MODIFIED Requirements

### Requirement: 追加の結果の表示
追加に成功したとき、WUIは、足したプロジェクトの参照を成功のトーストで表示し、プロジェクトの一覧を取り直さなければならない（MUST）。応答にwebhookの案内があれば、案内とコマンドは、トーストにせず、プロジェクトの一覧の下に表示する。
追加が拒まれたとき、WUIは、追加の欄に、拒まれた理由を表示しなければならない（MUST）。

#### Scenario: 中継を使う端末で足す
- **WHEN** `<agent.relay>`のある端末で、`gh:uskanda/notes`を追加する
- **THEN** 成功のトーストに`uskanda/notes`と「GitHub」のバッジ、「を追加しました」が表示され、一覧に`uskanda/notes`のカードが現れる。一覧の下には、`switchboard relay hook add gh:uskanda/notes`を実行するよう表示される

#### Scenario: 同じプロジェクトを足す
- **WHEN** 既にある`gh:uskanda/switchboard`を追加する
- **THEN** 追加の欄に、既にあることが表示される

### Requirement: 設定の画面の保存の結果
追加の指示の保存に成功したとき、WUIは、保存した欄の名前と、保存したことを、成功のトーストで表示しなければならない（MUST）。
保存の誤りは、トーストにせず、その欄の中に表示しなければならない（MUST）。

#### Scenario: 指示を保存する
- **WHEN** 設定の画面で、`gl:nas/sb`の計画の指示の欄に指示を書いて保存する
- **THEN** `nas/sb GitLabの計画の指示を保存しました`という成功のトーストが表示され、欄の中には知らせが残らない
