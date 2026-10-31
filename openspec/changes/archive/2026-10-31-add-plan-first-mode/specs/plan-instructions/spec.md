# Spec Delta

## Purpose

計画の実行に渡す利用者の追加の指示を、端末のローカルのファイルとして持ち、CLIと受信箱のAPIから安全に編集できるようにする。

## ADDED Requirements

### Requirement: 指示のファイルの置き場所
追加の指示は、設定ファイルと同じディレクトリの `<instructions/>` の下に置かなければならない（MUST）。
全体の指示は `<instructions/plan.md>` とする。
GitHubのプロジェクトの指示は `<instructions/gh/<owner>/<repo>/plan.md>` とする。
GitLabのプロジェクトの指示は `<instructions/gl/<group>/<project>/plan.md>` とする。GitLabのサブグループは、そのままディレクトリの階層にする。
`switchboard` は、指示のファイルを所有者だけが読み書きできる権限（0600）で、ディレクトリを0700で作らなければならない（MUST）。
`switchboard` は、指示を `config.yaml` に書いてはならない（MUST NOT）。

#### Scenario: GitLabのサブグループのプロジェクト
- **WHEN** `gl:nas/tools/switchboard-sandbox` の指示を保存する
- **THEN** 設定ファイルと同じディレクトリの `<instructions/gl/nas/tools/switchboard-sandbox/plan.md>` に、0600で書かれる

### Requirement: 指示の読み込み
計画の実行を始めるたびに、エージェントは、全体の指示とそのプロジェクトの指示を、ファイルから読み直さなければならない（MUST）。
両方あるとき、エージェントは、全体、プロジェクトの順につなげなければならない（MUST）。
ファイルが無いか、空白だけのとき、エージェントは、その指示を無いものとして扱わなければならない（MUST）。

#### Scenario: エージェントを止めずに指示を変える
- **WHEN** エージェントが動いている間に、プロジェクトの指示を書き換え、そのあとで計画の実行が始まる
- **THEN** その計画の実行には、書き換えたあとの指示が渡される

### Requirement: 指示の渡し方
追加の指示があるとき、エージェントは、計画の実行のsystem promptで、組み込みの指示のあとに「利用者の追加の指示」の節を作り、そこに指示を入れなければならない（MUST）。
その節には、組み込みの指示とぶつかるときは組み込みの指示を優先することを書く。
エージェントは、追加の指示を、実装の実行のsystem promptに入れてはならない（MUST NOT）。

#### Scenario: 指示が無い
- **WHEN** 全体の指示もプロジェクトの指示も無い状態で、計画の実行を始める
- **THEN** system promptに「利用者の追加の指示」の節は無い

#### Scenario: 承認のあとの実行
- **WHEN** プロジェクトの指示がある状態で、承認のあとの実装の実行を始める
- **THEN** system promptに追加の指示は入っていない

### Requirement: 保存の検査
指示を保存するとき、`switchboard` は、内容が32 KiB以下であることを確かめなければならない（MUST）。超えるときは保存せず、上限を超えたことを報告する。
指示を保存するとき、`switchboard` は、コメントの投稿の前と同じ秘密の検査をかけなければならない（MUST）。秘密らしいものがあるときは保存せず、そのことを報告する。
この検査は、CLIからの保存と、受信箱のAPIからの保存の両方に適用する。

#### Scenario: トークンを貼り付けた
- **WHEN** `ghp_` で始まるトークンを含む指示を保存しようとする
- **THEN** 保存されず、秘密らしいものがあると報告され、ファイルは前の内容のままである

### Requirement: 保存の記録
指示を保存したとき、`switchboard` は、監査ログに、日時、対象（全体かプロジェクト）、保存した内容のSHA-256を記録しなければならない（MUST）。
`switchboard` は、指示の内容そのものを監査ログに書いてはならない（MUST NOT）。

#### Scenario: Web表示から保存する
- **WHEN** Web表示から `gh:uskanda/switchboard` の指示を保存する
- **THEN** 監査ログに、日時、`gh:uskanda/switchboard`、内容のSHA-256が1行で記録される

### Requirement: CLIでの編集
`switchboard instructions edit` は、全体の指示を `$EDITOR` で開かなければならない（MUST）。`switchboard instructions edit <プロジェクト>` は、そのプロジェクトの指示を開く。
エディタを閉じたとき、内容が変わっていれば、`switchboard instructions edit` は保存しなければならない（MUST）。変わっていなければ保存しない。
設定に無いプロジェクトを指定したとき、`switchboard instructions edit` は、何もせずに誤りを報告しなければならない（MUST）。

#### Scenario: 設定に無いプロジェクト
- **WHEN** 設定に無い `gh:someone/other` を指定して `switchboard instructions edit` を実行する
- **THEN** エディタは開かず、0以外の終了コードで、設定に無いプロジェクトだと報告される
