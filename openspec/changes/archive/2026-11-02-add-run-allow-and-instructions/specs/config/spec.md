# Spec Delta

## ADDED Requirements

### Requirement: 追加で許可するツールの設定
設定は、`<agent.allow_tools_extra>` に、全プロジェクトへ足す許可の一覧を持てなければならない（MUST）。
プロジェクトの設定は、`<projects[].allow_tools_extra>` に、そのプロジェクトへ足す許可の一覧を持てなければならない（MUST）。
各項目は、Claude Codeの権限の規則の文字列（例 `Skill`、`Bash(openspec *)`）とする。
これらの項目が無い設定は、今までどおり読めなければならない（MUST）。

#### Scenario: 項目が無い設定
- **WHEN** `<agent.allow_tools_extra>` と `<projects[].allow_tools_extra>` のどちらも無い設定を読む
- **THEN** 誤りにならず、実行の許可は今までと同じである

### Requirement: 追加の許可の検査
設定を読むとき、`switchboard` は、追加の許可の各項目を検査しなければならない（MUST）。
次の項目は、設定の誤りとして、何も変更せずに終了し、誤りのある項目を報告する。
- 空の文字列か、空白だけの文字列
- 制限の無いBash（`Bash`、`Bash(*)`）
- 組み込みで拒否している操作に当たるもの（`git push`、`git rebase`、`gh`、`WebFetch`）
設定の読み直しで誤りが見つかったときは、前の設定のまま動き続ける。

#### Scenario: 制限の無いBash
- **WHEN** `<projects[].allow_tools_extra>` に `Bash(*)` がある設定で `switchboard agent run` を実行する
- **THEN** コマンドは0以外の終了コードで終わり、そのプロジェクトの `Bash(*)` が許されない項目だと報告する

#### Scenario: 拒否している操作
- **WHEN** `<agent.allow_tools_extra>` に `Bash(git push *)` がある設定を読む
- **THEN** 設定の誤りとして、`<agent.allow_tools_extra>` の `Bash(git push *)` が報告される

### Requirement: 追加の許可の読み直し
`<projects[].allow_tools_extra>` の変化は、`projects` の読み直しとともに反映しなければならない（MUST）。
`<agent.allow_tools_extra>` の変化は、`projects` 以外の変化として扱い、再起動まで反映しない。

#### Scenario: プロジェクトの追加の許可を足す
- **WHEN** `switchboard agent run` が動いている間に、あるプロジェクトの `<projects[].allow_tools_extra>` に `Skill` を足して保存する
- **THEN** 10秒ほどのうちに、そのあとで始まるそのプロジェクトの実行では `Skill` が許される
