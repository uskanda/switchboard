# Spec Delta

## ADDED Requirements

### Requirement: 実装の実行だけの指示の読み書き
APIは、指示の読み出しで、対象ごとに実装の実行だけの指示の項目も返さなければならない（MUST）。その項目の種別は `implement` とする。
APIは、種別 `implement` の保存を受け付け、`implement.md` に書かなければならない（MUST）。保存の検査と誤りは、ほかの種別と同じにする。

#### Scenario: 実装の実行だけの指示を保存する
- **WHEN** 種別 `implement` で `gh:uskanda/switchboard` の指示を保存する
- **THEN** `<instructions/gh/uskanda/switchboard/implement.md>` が書き換わり、同じ対象の `plan.md` と `run.md` は変わらない
