# Spec Delta

## ADDED Requirements

### Requirement: agent.retentionで実行ディレクトリの保持期間を決める
設定は `agent.retention` を受け付けなければならない（MUST）。省いたときは720hとし、`0` は消さないことを意味しなければならない（MUST）。

#### Scenario: 省いた設定
- **WHEN** 設定に `agent.retention` が無い
- **THEN** 保持期間は720hになる

#### Scenario: 0
- **WHEN** `agent.retention: 0` と書く
- **THEN** 実行ディレクトリは消されない

### Requirement: 同じクレデンシャル名になるtoken_refを拒む
異なる2つの `token_ref` が同じsystemdのクレデンシャル名になるとき、設定の読み込みと `service install` は、その組を名指しして誤りにしなければならない（MUST）。

#### Scenario: a/bとa_b
- **WHEN** 設定に `token_ref: a/b` と `token_ref: a_b` がある
- **THEN** 設定の読み込みは、2つの名前を挙げて誤りを返す
