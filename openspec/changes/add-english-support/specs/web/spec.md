# Spec Delta

## MODIFIED Requirements

### Requirement: 理由のバッジの名前
日本語で表示するとき、WUIは、理由のバッジに、理由の語に続けて日本語の短い名前を出さなければならない（MUST）。名前は、`question` を質問、`verify` を確認、`plan-questions` を計画の質問、`plan` を計画の承認とする。失敗の理由は、`error` を実行の失敗、`permission` を権限の不足、`push` をpushの失敗、`ci` をCIの失敗、`conflict` を衝突とする。
英語で表示するとき、WUIは、語だけでは分かりにくい理由にだけ、理由の語に続けて英語の短い名前を出さなければならない（MUST）。名前は、`verify` をchecks、`plan` をapproval、`error` をrun failed、`permission` をdenied、`push` をfailed、`ci` をfailedとする。`question`、`plan-questions`、`conflict` には名前を付けない。
名前の無い理由では、WUIは語だけを出さなければならない（MUST）。

#### Scenario: CIの失敗の待ち
- **WHEN** 理由が `ci` の待ちを一覧に出す
- **THEN** バッジに `ci` と「CIの失敗」が並ぶ

#### Scenario: 英語のCIの失敗の待ち
- **WHEN** 英語で表示しているWUIで、理由が `ci` の待ちを一覧に出す
- **THEN** バッジに `ci` と `failed` が並ぶ

#### Scenario: 英語の質問の待ち
- **WHEN** 英語で表示しているWUIで、理由が `question` の待ちを一覧に出す
- **THEN** バッジには `question` だけが出る

### Requirement: 設定の画面の誤りと権限
設定の画面の読み直しが成功したとき、WUIは、前の読み込みの誤りを消さなければならない（MUST）。
WUIは、ログインした人が管理者かどうかによらず、設定の画面の表示の節を出さなければならない（MUST）。
ログインが有効で、ログインした人が管理者でないとき、WUIは、設定の画面の表示の節の下に`プロジェクトと追加の指示を変えられるのは管理者だけです`を出さなければならない（MUST）。このとき、WUIはプロジェクトと追加の指示を読んではならない（MUST NOT）。
WUIは、ログインの状態が分かるまで、設定の画面で何も読んではならない（MUST NOT）。

#### Scenario: 読めなかった後に読み直す
- **WHEN** 追加の指示を読めなかった後、プロジェクトを追加して読み直しが成功する
- **THEN** 誤りの表示は消える

#### Scenario: 管理者でない人がURLで開く
- **WHEN** 管理者でない人が `/settings` を開く
- **THEN** 表示の節と、管理者だけが変えられることの文が出て、`/api/instructions` と `/api/projects` は呼ばれない

#### Scenario: 管理者でない人がヘッダから開く
- **WHEN** ログインが有効で、管理者でない人が一覧を開く
- **THEN** ヘッダに設定の画面へのリンクがあり、押すと表示の節が出る
