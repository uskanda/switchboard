# Spec Delta

## Purpose

トークンを置く保管先を、端末のOSと環境ごとに定める。Secret Serviceの無いLinuxでも、トークンを平文で置かずに、CLIと常駐のサービスの両方が読み書きできるようにする。

## ADDED Requirements

### Requirement: OSの保管先
Linuxで動くとき、`switchboard` は、Secret ServiceをOSの保管先として使わなければならない（MUST）。
macOSで動くとき、`switchboard` は、ログインキーチェーンをOSの保管先として使わなければならない（MUST）。項目は、サービス名を `token_ref`、アカウント名を `token` とする汎用パスワードである。

#### Scenario: Macのキーチェーンから読む
- **WHEN** macOSの端末で、ログインキーチェーンにサービス名 `switchboard/github`、アカウント名 `token` の項目があり、設定の `token_ref` が `switchboard/github` である
- **THEN** その項目の値がトークンとして使われる

### Requirement: ユーザーの範囲の暗号化クレデンシャル
Linuxで、Secret Serviceに接続できないとき、`switchboard` は、`~/.config/credstore.encrypted/` のユーザーの範囲の暗号化クレデンシャルを、秘密の保管先として使わなければならない（MUST）。
ファイルの名前は、`token_ref` から「トークンの取り出し」と同じ規則で作ったクレデンシャルの名前とする。
`switchboard` は、このファイルを `systemd-creds decrypt --user` で読み、`systemd-creds encrypt --user` で書かなければならない（MUST）。
`switchboard` は、トークンの値を、コマンドの引数、表示、平文のファイルに出してはならない（MUST NOT）。

#### Scenario: ヘッドレスの端末で読む
- **WHEN** Secret Serviceの無いsystemd 259の端末で、`~/.config/credstore.encrypted/switchboard_github` があり、設定の `token_ref` が `switchboard/github` である
- **THEN** そのファイルを復号した値がトークンとして使われる

#### Scenario: 別のユーザーのファイル
- **WHEN** 別のOSユーザーが `--user` で暗号化したファイルを、自分の `~/.config/credstore.encrypted/` に複製する
- **THEN** 復号に失敗し、コマンドは0以外の終了コードで終わり、トークンの値は出力に現れない

### Requirement: 保管先への書き込み
秘密を書き込むコマンド（`relay token add` など）は、OSの保管先に接続できるとき、OSの保管先に書かなければならない（MUST）。
Linuxで、Secret Serviceに接続できないとき、秘密を書き込むコマンドは、ユーザーの範囲の暗号化クレデンシャルに書かなければならない（MUST）。

#### Scenario: ヘッドレスの端末で書く
- **WHEN** Secret Serviceの無いsystemd 259の端末で、`switchboard relay token add host1` を実行する
- **THEN** 新しいトークンは `~/.config/credstore.encrypted/` に `--user` で暗号化されて置かれ、平文のファイルは作られない

### Requirement: 古いsystemdでの保管先
Linuxで、Secret Serviceに接続できず、systemdが256より古いとき、秘密を読み書きするコマンドは、何をすればよいかを案内して失敗しなければならない（MUST）。案内には、Secret Serviceを使えるようにすること、systemd 256以上に上げること、開発のときだけ `allow_env` を使えることを含める。

#### Scenario: Ubuntu 24.04のヘッドレスの端末
- **WHEN** Secret Serviceの無いsystemd 255の端末で、`switchboard relay token add host1` を実行する
- **THEN** 何も書かれず、コマンドは0以外の終了コードで終わり、systemd 256以上が要ることが案内される

### Requirement: トークンが見つからないときの案内
トークンがどの保管先にも見つからないとき、`switchboard` は、その端末で使っている保管先に合った置き方を案内しなければならない（MUST）。
Secret Serviceを使うLinuxでは `secret-tool store`、ユーザーの範囲の暗号化クレデンシャルを使うLinuxでは `systemd-creds encrypt --user`、macOSでは `security add-generic-password` を案内する。
案内に、トークンの値を含めてはならない（MUST NOT）。

#### Scenario: Macで見つからない
- **WHEN** macOSの端末で、設定の `token_ref` が `switchboard/github` で、キーチェーンに項目が無く、`allow_env` も無い
- **THEN** コマンドは0以外の終了コードで終わり、`security add-generic-password` で `switchboard/github` を置く方法が案内される

#### Scenario: ヘッドレスの端末で見つからない
- **WHEN** Secret Serviceの無いsystemd 259の端末で、`switchboard/github` のファイルが無く、`allow_env` も無い
- **THEN** コマンドは0以外の終了コードで終わり、`systemd-creds encrypt --user` でファイルを置く方法が案内され、`secret-tool` は案内されない
