# Spec Delta

## ADDED Requirements

### Requirement: 追加の指示の読み出し
APIは、全体の指示と、設定にあるプロジェクトごとの指示を返さなければならない（MUST）。
各項目は、対象（全体か、`gh:owner/repo` の形のプロジェクト）、今の内容、`switchboard inbox serve` が動く端末の名前を持つ。ファイルが無い指示は、空の内容として返す。

#### Scenario: プロジェクトの指示だけがある
- **WHEN** 全体の指示が無く、`gh:uskanda/switchboard` の指示だけがある状態で、指示を求める
- **THEN** 全体の指示は空で、`gh:uskanda/switchboard` の指示は今の内容で返り、ほかのプロジェクトは空で返る

### Requirement: 追加の指示の保存
APIは、指定した対象の指示を保存しなければならない（MUST）。保存の前の検査と記録は、`plan-instructions` の保存と同じにする。
検査で拒んだとき、APIは保存せずに、大きさの上限を超えたことか、秘密らしいものがあることを区別できる誤りを返さなければならない（MUST）。
設定に無いプロジェクトを指定したとき、APIは保存せずに誤りを返さなければならない（MUST）。

#### Scenario: 大きすぎる指示
- **WHEN** 40 KiBの指示を保存しようとする
- **THEN** 保存されず、大きさの上限を超えたことを示す誤りが返る

#### Scenario: 設定に無いプロジェクト
- **WHEN** 設定に無い `gh:someone/other` の指示を保存しようとする
- **THEN** 保存されず、誤りが返る
