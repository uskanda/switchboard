## MODIFIED Requirements

### Requirement: トークンの取り出し
`switchboard` は、`token_ref` に対応するsystemdのクレデンシャルがあれば、その中身をトークンとして使わなければならない（MUST）。クレデンシャルは、環境変数 `CREDENTIALS_DIRECTORY` が指すディレクトリから探す。
クレデンシャルの名前は、`token_ref` の英数字以外の文字を `_` に置き換えた文字列とする。
クレデンシャルが無いとき、`switchboard` は、`token_ref` が指す項目をSecret Serviceから取り出してトークンとして使わなければならない（MUST）。
設定の `secrets` の `allow_env` が `true` の間、Secret Serviceに項目が無いとき、`switchboard` は環境変数からトークンを読まなければならない（MUST）。
変数の名前は `SWITCHBOARD_TOKEN_` に、`token_ref` を英大文字にして英数字以外を `_` に置き換えた文字列を続けたものとする。

#### Scenario: Secret Serviceから取り出す
- **WHEN** Secret Serviceに `switchboard/github` の項目があり、設定の `token_ref` が `switchboard/github` である
- **THEN** その項目の値がトークンとして使われる

#### Scenario: 開発時の環境変数
- **WHEN** `allow_env: true` の設定で、Secret Serviceに項目が無く、`SWITCHBOARD_TOKEN_SWITCHBOARD_GITHUB` が設定されている
- **THEN** その環境変数の値がトークンとして使われる

#### Scenario: 環境変数が許されていない
- **WHEN** `allow_env` が無い設定で、Secret Serviceに項目が無い
- **THEN** コマンドは0以外の終了コードで終わり、取り出せない `token_ref` を報告する。環境変数は読まれない

#### Scenario: systemdのクレデンシャル
- **WHEN** systemdのサービスとして動き、`$CREDENTIALS_DIRECTORY/switchboard_github` がある
- **THEN** その中身がトークンとして使われ、Secret Serviceには問い合わせない
