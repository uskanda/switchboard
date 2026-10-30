## Purpose

エージェントを、ログインしていなくても動き続けるsystemdのサービスとして常駐させる。sudoを要する操作は、実行する前に示し、利用者の確認を取る。

## ADDED Requirements

### Requirement: サービスの設置
`switchboard service install` は、エージェントをsystemdのシステムサービスとして設置し、有効にして起動しなければならない（MUST）。unitの `User=` には、今のOSユーザーを書く。
サービスは、設置したときの設定ファイルの絶対パスを `--config` に渡して `switchboard agent run` を実行する。
サービスは、異常に終わったら自動で再起動しなければならない（MUST）。
設置済みのサービスがあるとき、`service install` は、unitとクレデンシャルを置き直して、サービスを再起動しなければならない（MUST）。

#### Scenario: 初めて設置する
- **WHEN** サービスが無い端末で、`switchboard service install` を実行して確認に `y` と答える
- **THEN** unitとクレデンシャルが置かれ、サービスが有効になって動き出す

#### Scenario: プロセスが落ちる
- **WHEN** 動いているサービスのエージェントのプロセスが、シグナルで止められる
- **THEN** systemdがエージェントを再起動する

### Requirement: クレデンシャル
`service install` は、この端末の担当プロジェクトのエージェント用トークンだけを、systemdのクレデンシャルにしなければならない（MUST）。対象は、フォージの `token_ref` と、プロジェクトの上書きの `token_ref` である。クレデンシャルは、ホストの鍵で暗号化する。
`service install` は、受信箱のトークン（`inbox_token_ref`）をクレデンシャルにしてはならない（MUST NOT）。
トークンの値を、コマンドの引数、表示、平文のファイルに出してはならない（MUST NOT）。

#### Scenario: 置くクレデンシャル
- **WHEN** GitHubとGitLabのプロジェクトを1つずつ担当する端末で `service install` を実行する
- **THEN** `token_ref` の2つだけがクレデンシャルになり、`inbox_token_ref` はならない

### Requirement: sudoの前の確認
`service install` と `service uninstall` は、sudoで実行するコマンドを、対象のパスとともに全部表示してから、`[y/N]` で確認を取らなければならない（MUST）。
確認に `y` と答えなかったとき、何も実行してはならない（MUST NOT）。
`--dry-run` が付いたとき、表示だけをして終わらなければならない（MUST）。
標準入力が端末でないとき、`--yes` が無ければ、何も実行せずに理由を報告しなければならない（MUST）。
sudoは、表示したコマンドごとに個別に呼ばなければならない（MUST）。

#### Scenario: 確認を断る
- **WHEN** `switchboard service install` を実行し、確認に `n` と答える
- **THEN** sudoは呼ばれず、unitとクレデンシャルは置かれない

#### Scenario: パイプからの実行
- **WHEN** 標準入力をパイプにして、`--yes` を付けずに `switchboard service install` を実行する
- **THEN** 何も実行されず、`--yes` が要ると報告される

### Requirement: サービスの撤去
`switchboard service uninstall` は、サービスを止めて無効にし、unitとクレデンシャルを消さなければならない（MUST）。

#### Scenario: 撤去する
- **WHEN** 設置済みの端末で `switchboard service uninstall` を実行し、確認に `y` と答える
- **THEN** サービスが止まり、unitとクレデンシャルが消える

### Requirement: 状態の表示
`switchboard service status` は、sudoを使わずに、unitとクレデンシャルの有無、サービスが有効か、動いているかを表示しなければならない（MUST）。

#### Scenario: 設置済みの状態
- **WHEN** 設置済みで動いている端末で `switchboard service status` を実行する
- **THEN** unitがあり、有効で、動いていると表示される
