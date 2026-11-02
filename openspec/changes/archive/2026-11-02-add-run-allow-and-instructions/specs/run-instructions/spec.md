# Spec Delta

## Purpose
ファイルを書く実行に渡す利用者の追加の指示を、計画用の指示とは別のローカルのファイルとして持ち、計画用と同じ方法で安全に編集できるようにする。

## ADDED Requirements

### Requirement: 実装用の指示のファイルの置き場所
実装用の追加の指示は、計画用の指示と同じディレクトリに置かなければならない（MUST）。
全体の指示は `<instructions/run.md>` とする。
GitHubのプロジェクトの指示は `<instructions/gh/<owner>/<repo>/run.md>` とする。
GitLabのプロジェクトの指示は `<instructions/gl/<group>/<project>/run.md>` とする。
ファイルとディレクトリの権限は、計画用の指示と同じにする。

#### Scenario: GitHubのプロジェクトの実装用の指示
- **WHEN** `gh:uskanda/switchboard` の実装用の指示を保存する
- **THEN** 設定ファイルと同じディレクトリの `<instructions/gh/uskanda/switchboard/run.md>` に、0600で書かれる

### Requirement: 実装用の指示の読み込み
ファイルを書く実行を始めるたびに、エージェントは、全体とそのプロジェクトの実装用の指示を、ファイルから読み直さなければならない（MUST）。
両方あるとき、エージェントは、全体、プロジェクトの順につなげなければならない（MUST）。
ファイルが無いか、空白だけのとき、エージェントは、その指示を無いものとして扱わなければならない（MUST）。
ファイルを書く実行とは、直接の実装、承認のあとの実装、CIとpushの失敗を直す実行、baseとの衝突を直す実行である。

#### Scenario: 動いている間に指示を変える
- **WHEN** エージェントが動いている間に全体の実装用の指示を書き換え、そのあとで直す実行が始まる
- **THEN** その直す実行には、書き換えたあとの指示が渡される

### Requirement: 実装用の指示の渡し方
実装用の指示があるとき、エージェントは、ファイルを書く実行のsystem promptで、組み込みの指示のあとに「利用者の追加の指示」の節を作らなければならない（MUST）。
その節には、組み込みの指示とぶつかるときは組み込みの指示を優先することを書く。
エージェントは、実装用の指示を、計画の実行のsystem promptに入れてはならない（MUST NOT）。

#### Scenario: 承認のあとの実装
- **WHEN** プロジェクトの実装用の指示がある状態で、承認のあとの実装の実行を始める
- **THEN** system promptの終わりに「利用者の追加の指示」の節があり、実装用の指示が入っている

#### Scenario: 計画の実行
- **WHEN** 実装用の指示だけがある状態で、計画の実行を始める
- **THEN** system promptに「利用者の追加の指示」の節は無い

#### Scenario: 指示が無い
- **WHEN** 実装用の指示が全体とプロジェクトのどちらにも無い状態で、直接の実装の実行を始める
- **THEN** system promptに「利用者の追加の指示」の節は無い

### Requirement: 実装用の指示の保存と記録
実装用の指示の保存には、計画用の指示と同じ検査（32 KiBの上限、秘密の検査）をかけなければならない（MUST）。
保存したとき、`switchboard` は、監査ログに、日時、対象、指示の種別（計画用と実装用の別）、内容のSHA-256を記録しなければならない（MUST）。
`switchboard` は、指示の内容そのものを監査ログに書いてはならない（MUST NOT）。

#### Scenario: 実装用の指示にトークンを貼る
- **WHEN** `glpat-` で始まるトークンを含む実装用の指示を保存しようとする
- **THEN** 保存されず、秘密らしいものがあると報告され、ファイルは前の内容のままである

### Requirement: CLIでの実装用の指示の編集
`switchboard instructions edit --run` は、全体の実装用の指示を `$EDITOR` で開かなければならない（MUST）。
`switchboard instructions edit --run <プロジェクト>` は、そのプロジェクトの実装用の指示を開く。
`--run` が無いときの動きは、今までどおり計画用の指示の編集とする。
保存するかどうかの判断と、設定に無いプロジェクトの扱いは、計画用の指示の編集と同じにする。

#### Scenario: 実装用の指示を開く
- **WHEN** `switchboard instructions edit --run gh:uskanda/switchboard` を実行する
- **THEN** `<instructions/gh/uskanda/switchboard/run.md>` がエディタで開き、計画用の `plan.md` は開かない
