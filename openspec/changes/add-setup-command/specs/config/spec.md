# Spec Delta

## ADDED Requirements

### Requirement: 準備のコマンドの設定
プロジェクトの設定は、`<projects[].setup_command>` に、準備のコマンドを1つ持てなければならない（MUST）。
準備のコマンドは、シェルに渡す1行の文字列とする。
この項目が無い設定は、今までどおり読めなければならない（MUST）。
設定は、全プロジェクトに共通の準備のコマンドを持たない。

#### Scenario: 項目が無い設定
- **WHEN** どのプロジェクトにも `<projects[].setup_command>` が無い設定を読む
- **THEN** 誤りにならず、実行は今までと同じである

#### Scenario: 項目のある設定
- **WHEN** あるプロジェクトに `setup_command: pnpm install --frozen-lockfile` を書いた設定を読む
- **THEN** 誤りにならず、そのプロジェクトの準備のコマンドは `pnpm install --frozen-lockfile` である

#### Scenario: agentの下に書く
- **WHEN** `agent:` の下に `setup_command` を書いた設定で `switchboard agent run` を実行する
- **THEN** コマンドは0以外の終了コードで終わり、`setup_command` を未知のキーとして報告する

### Requirement: 準備のコマンドの読み直し
`<projects[].setup_command>` の変化は、`projects` の読み直しとともに反映しなければならない（MUST）。

#### Scenario: 準備のコマンドを足す
- **WHEN** `switchboard agent run` が動いている間に、あるプロジェクトに `setup_command` を足して保存する
- **THEN** 10秒ほどのうちに、そのあとで始まるそのプロジェクトの実行では準備のコマンドが実行される
