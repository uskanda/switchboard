# run-instructions Specification

## Purpose
ファイルを書く実行に渡す利用者の追加の指示を、計画用の指示とは別のローカルのファイルとして持ち、計画用と同じ方法で安全に編集できるようにする。

## Requirements

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

### Requirement: 実行の開始の記録
claudeを起動して実行を始めるとき、エージェントは、監査ログに `run.started` を1行記録しなければならない（MUST）。
その行には、issue、実行のID、実行の種類（`direct`、`plan`、`approved`、`fix`）、足した許可の一覧、全体とプロジェクトの指示のファイルごとのSHA-256を入れる。
実行の種類が `fix` のとき、その行には、直す原因（`ci`、`push`、`conflict`）を `run.fix` と同じ値で入れる。
足した許可の一覧は、実際にallowへ足した順に並べ、前後の空白を除き、既定のallowや前の項目と重なるものを省いたものとする。
指示のハッシュは、ファイルの中身そのもののSHA-256とし、保存のときに `instructions.saved` へ記録する値と同じにする。
ファイルが無いか、空白だけか、読めないとき、そのハッシュは空の文字列とする。

#### Scenario: 直接の実装
- **WHEN** 全体とプロジェクトの `allow_tools_extra` と、全体とプロジェクトの `run.md` がある状態で、直接の実装の実行を始める
- **THEN** 監査ログに `kind` が `direct` の `run.started` が1行ある。`allow` は全体の分のあとにプロジェクトの分を足した一覧である。`instructions` の値は、保存した `instructions.saved` の `sha256` と一致する

#### Scenario: pushを拒まれたあとの直す実行
- **WHEN** pushを拒まれ、直す実行が始まる
- **THEN** `run.fix` のあとに、新しい実行のIDで、`kind` が `fix`、`cause` が `push` の `run.started` が記録される

### Requirement: 計画の実行の開始の記録
計画の実行を始めるとき、エージェントは、`run.started` の足した許可を空の一覧にしなければならない（MUST）。
その行の指示のハッシュは、計画用の指示（`plan.md`）のものにする。

#### Scenario: 計画の実行
- **WHEN** 追加の許可と計画用の指示がある状態で、計画の実行を始める
- **THEN** `run.started` の `kind` は `plan`、`allow` は `[]` で、`instructions` の値は `plan.md` のハッシュである

### Requirement: 実行の開始の記録に指示の内容を書かない
エージェントは、指示の内容そのものを `run.started` に書いてはならない（MUST NOT）。

#### Scenario: 指示の中身
- **WHEN** 実装用の指示がある状態で、実行を始める
- **THEN** 監査ログのどの行にも、指示の文章が現れない
