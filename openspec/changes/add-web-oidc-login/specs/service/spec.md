# Spec Delta

## ADDED Requirements

### Requirement: 受信箱のサービスのclient secret
`<inbox.auth>` があるとき、`service install --inbox` は、client secretをクレデンシャルにしなければならない（MUST）。client secretは、`<client_secret_ref>` の名前で読む。クレデンシャルは、ホストの鍵で暗号化したsystemdのクレデンシャルとする。
client secretの値を、コマンドの引数、表示、平文のファイルに出してはならない（MUST NOT）。

#### Scenario: 認証を設定して受信箱のサービスを設置する
- **WHEN** `<inbox.auth>` のある設定で `service install --inbox` を実行する
- **THEN** 表示されるsudoのコマンドに、client secretのクレデンシャルを置く行が含まれ、値は表示されない
