# Spec Delta

## ADDED Requirements

### Requirement: 実装の実行だけの指示のファイル
実装の実行だけの指示は、計画用と実装用の指示と同じディレクトリに置かなければならない（MUST）。
全体の指示は `<instructions/implement.md>` とする。
GitHubのプロジェクトの指示は `<instructions/gh/<owner>/<repo>/implement.md>` とする。
GitLabのプロジェクトの指示は `<instructions/gl/<group>/<project>/implement.md>` とする。
保存の検査（32 KiBの上限、秘密の検査）と、監査ログの `instructions.saved` は、ほかの種類と同じにする。記録する種別は `implement` とする。

#### Scenario: プロジェクトの実装の実行だけの指示を保存する
- **WHEN** `gh:uskanda/switchboard` の実装の実行だけの指示を保存する
- **THEN** `<instructions/gh/uskanda/switchboard/implement.md>` に0600で書かれる。`instructions.saved` の `kind` は `implement` である

### Requirement: 実装の実行だけの指示を渡す実行
エージェントは、直接の実装と承認のあとの実装を始めるたびに、全体とプロジェクトの実装の実行だけの指示をファイルから読み直さなければならない（MUST）。
読んだ指示は、`run.md` の実装用の指示のあとにつなげ、同じ「利用者の追加の指示」の節に入れなければならない（MUST）。
エージェントは、実装の実行だけの指示を、計画の実行と直す実行（CI、push、baseとの衝突）に入れてはならない（MUST NOT）。

#### Scenario: 直接の実装
- **WHEN** 全体の `run.md` とプロジェクトの `implement.md` がある状態で、直接の実装を始める
- **THEN** 利用者の追加の指示の節に、`run.md` の内容のあとに `implement.md` の内容が入っている

#### Scenario: CIの失敗を直す実行
- **WHEN** `implement.md` だけがある状態で、CIの失敗を直す実行を始める
- **THEN** system promptに `implement.md` の内容は無い

### Requirement: 実装の実行だけの指示のハッシュの記録
エージェントは、`run.started` に、実装の実行だけの指示の全体とプロジェクトのSHA-256を、`implement` の鍵で記録しなければならない（MUST）。
計画の実行と直す実行では、この鍵の値は空の文字列とする。

#### Scenario: 承認のあとの実装
- **WHEN** プロジェクトの `implement.md` がある状態で、承認のあとの実装を始める
- **THEN** `run.started` の `<implement.project>` は、保存したときの `instructions.saved` の `sha256` と一致する

### Requirement: CLIでの実装の実行だけの指示の編集
`switchboard instructions edit --implement [<プロジェクト>]` は、実装の実行だけの指示を `$EDITOR` で開かなければならない（MUST）。
`--run` と `--implement` を同時に指定したとき、CLIは誤りで終わらなければならない（MUST）。

#### Scenario: 実装の実行だけの指示を開く
- **WHEN** `switchboard instructions edit --implement gh:uskanda/switchboard` を実行する
- **THEN** `<instructions/gh/uskanda/switchboard/implement.md>` がエディタで開く
