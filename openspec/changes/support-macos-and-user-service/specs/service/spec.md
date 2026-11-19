# Spec Delta

## ADDED Requirements

### Requirement: ユーザーunitの設置
Linuxで `--user` が付いたとき、`service install` は、エージェントをsystemdのユーザーunitとして `~/.config/systemd/user/` に設置し、`systemctl --user` で有効にして起動しなければならない（MUST）。
Linuxで `--user` が付いたとき、`service install` は、`loginctl enable-linger` で今のOSユーザーのlingerを有効にしなければならない（MUST）。
`--user` が付いたとき、`service install`、`service uninstall`、`service status` は、sudoを呼んではならない（MUST NOT）。
systemdが256より古いとき、`service install --user` は、何も実行せずに、systemd 256以上が要ることを報告しなければならない（MUST）。
同じ名前のシステムサービスのunitがあるとき、`service install --user` は、何も実行せずに、システムサービスの撤去を案内しなければならない（MUST）。同じ名前のユーザーunitがあるとき、`--user` の無い `service install` も、同じように断る。
`--user` は、`--relay`、`--inbox`、`--web` と組み合わせられなければならない（MUST）。unitの名前と、unitが実行するコマンドは、システムサービスと同じとする。
`--user` が付いたとき、`service uninstall` は、ユーザーunitを止めて無効にし、unitとクレデンシャルを消さなければならない（MUST）。lingerは無効にしない。
`--user` が付いたとき、`service status` は、ユーザーunitとクレデンシャルの有無、有効か、動いているか、lingerが有効かを表示しなければならない（MUST）。

#### Scenario: ユーザーunitを設置する
- **WHEN** systemd 259の端末で `switchboard service install --user` を実行し、確認に `y` と答える
- **THEN** sudoは呼ばれず、`~/.config/systemd/user/switchboard-agent.service` とクレデンシャルが置かれ、lingerが有効になり、ユーザーunitが動き出す

#### Scenario: 古いsystemd
- **WHEN** systemd 255の端末で `switchboard service install --user` を実行する
- **THEN** 何も実行されず、コマンドは0以外の終了コードで終わり、systemd 256以上が要ると報告される

#### Scenario: システムサービスが残っている
- **WHEN** `/etc/systemd/system/switchboard-agent.service` がある端末で `switchboard service install --user` を実行する
- **THEN** 何も実行されず、コマンドは0以外の終了コードで終わり、`switchboard service uninstall` で撤去するよう案内される

#### Scenario: ログアウトしたあと
- **WHEN** ユーザーunitを設置した端末で、利用者がすべてのセッションからログアウトする
- **THEN** ユーザーunitのエージェントは動き続ける

#### Scenario: 受信箱のユーザーunit
- **WHEN** `switchboard service install --user --inbox` を実行し、確認に `y` と答える
- **THEN** `~/.config/systemd/user/switchboard-inbox.service` が置かれ、各フォージの `inbox_token_ref` がユーザーの範囲のクレデンシャルになる

#### Scenario: ユーザーunitを撤去する
- **WHEN** ユーザーunitを設置した端末で `switchboard service uninstall --user` を実行し、確認に `y` と答える
- **THEN** ユーザーunitが止まり、unitとクレデンシャルが消え、sudoは呼ばれない

### Requirement: macOSのLaunchAgentの設置
macOSで `service install` を実行したとき、`service install` は、エージェントのLaunchAgentのplistを `~/Library/LaunchAgents/` に置き、`launchctl bootstrap gui/<uid>` で読み込まなければならない（MUST）。
LaunchAgentは、設置したときの設定ファイルの絶対パスを `--config` に渡して `switchboard agent run` を実行し、異常に終わったら自動で再起動しなければならない（MUST）。
macOSでは、`service install` は、`--user` の有無にかかわらずLaunchAgentを設置しなければならない（MUST）。
macOSでは、`service install` は、クレデンシャルを作ってはならない（MUST NOT）。LaunchAgentのエージェントは、実行時にログインキーチェーンからトークンを読む。
macOSで、対象の `token_ref` のどれかをキーチェーンから読めないとき、`service install` は、何も実行せずに、読めない `token_ref` を報告しなければならない（MUST）。
macOSでは、`service install`、`service uninstall`、`service status` は、sudoを呼んではならない（MUST NOT）。
設置済みのLaunchAgentがあるとき、`service install` は、plistを置き直して読み込み直さなければならない（MUST）。
`--relay`、`--inbox`、`--web` が付いたとき、`service install` は、それぞれのサービスを同じ形のLaunchAgentとして設置しなければならない（MUST）。
macOSで `service uninstall` を実行したとき、`service uninstall` は、`launchctl bootout` でLaunchAgentを止めて、plistを消さなければならない（MUST）。
macOSで `service status` を実行したとき、`service status` は、plistの有無と、`launchctl print` で得た読み込みと実行の状態を表示しなければならない（MUST）。

#### Scenario: Macに設置する
- **WHEN** macOSの端末で `switchboard service install` を実行し、確認に `y` と答える
- **THEN** sudoは呼ばれず、plistが `~/Library/LaunchAgents/` に置かれ、`launchctl bootstrap gui/<uid>` で読み込まれ、エージェントが動き出す

#### Scenario: Macでトークンを渡さない
- **WHEN** macOSの端末で `switchboard service install --dry-run` を実行する
- **THEN** 表示される手順に、トークンを暗号化する手順もトークンを書いたファイルも無い

#### Scenario: Macのキーチェーンに無いトークン
- **WHEN** macOSの端末で、キーチェーンに `switchboard/github` の項目が無い設定で `switchboard service install` を実行する
- **THEN** 何も実行されず、コマンドは0以外の終了コードで終わり、`switchboard/github` を読めないと報告される

#### Scenario: Macから撤去する
- **WHEN** LaunchAgentを設置したmacOSの端末で `switchboard service uninstall` を実行し、確認に `y` と答える
- **THEN** `launchctl bootout` でエージェントが止まり、plistが消える

#### Scenario: Macの状態を表示する
- **WHEN** LaunchAgentを設置して動いているmacOSの端末で `switchboard service status` を実行する
- **THEN** plistがあり、読み込まれていて、動いていると表示される

### Requirement: 対応していないOSでの設置
LinuxとmacOS以外のOSで `service install`、`service uninstall`、`service status` を実行したとき、`switchboard` は、何も実行せずに、そのOSに対応していないことを報告しなければならない（MUST）。

#### Scenario: FreeBSDで設置する
- **WHEN** FreeBSDの端末で `switchboard service install` を実行する
- **THEN** 何も実行されず、コマンドは0以外の終了コードで終わり、このOSに対応していないと報告される

## MODIFIED Requirements

### Requirement: sudoの前の確認
`service install` と `service uninstall` は、実行するコマンドを、対象のパスとともに全部表示してから、`[y/N]` で確認を取らなければならない（MUST）。sudoで実行するコマンドも、sudoを使わないコマンド（`--user` とmacOSの手順）も、同じように扱う。
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

#### Scenario: ユーザーunitの確認を断る
- **WHEN** `switchboard service install --user` を実行し、表示された手順に確認で `n` と答える
- **THEN** `systemctl --user` と `loginctl` は呼ばれず、unitとクレデンシャルは置かれない

#### Scenario: Macのdry-run
- **WHEN** macOSの端末で `switchboard service install --dry-run` を実行する
- **THEN** plistのパスと `launchctl` のコマンドが表示され、何も実行されない

### Requirement: クレデンシャル
`service install` は、この端末の担当プロジェクトのエージェント用トークンだけを、systemdのクレデンシャルにしなければならない（MUST）。対象は、フォージの `token_ref` と、プロジェクトの上書きの `token_ref` である。設定の `agent` に `<relay>` があれば、その `token_ref`（中継のbearerトークン）も対象にする。設定に `<notify.telegram>` があれば、その `token_ref`（Telegramのbotのトークン）も対象にする。
システムサービスのとき、`service install` は、クレデンシャルをホストの鍵で暗号化しなければならない（MUST）。
`--user` が付いたとき、`service install` は、クレデンシャルを `systemd-creds encrypt --user` でユーザーの範囲に暗号化し、`~/.config/credstore.encrypted/` に置かなければならない（MUST）。ユーザーunitは、それを `LoadCredentialEncrypted=` で読む。
`service install` は、受信箱のトークン（`inbox_token_ref`）をエージェントのクレデンシャルにしてはならない（MUST NOT）。
トークンの値を、コマンドの引数、表示、平文のファイルに出してはならない（MUST NOT）。

#### Scenario: 置くクレデンシャル
- **WHEN** GitHubとGitLabのプロジェクトを1つずつ担当する端末で `service install` を実行する
- **THEN** `token_ref` の2つだけがクレデンシャルになり、`inbox_token_ref` はならない

#### Scenario: 中継モードのエージェント
- **WHEN** `agent` に `<relay>` がある設定で `service install` を実行する
- **THEN** 中継のbearerトークンもクレデンシャルになる

#### Scenario: 通知するエージェント
- **WHEN** `<notify.telegram>` がある設定で `service install` を実行する
- **THEN** Telegramのbotのトークンもクレデンシャルになり、`inbox_token_ref` はならない

#### Scenario: ユーザーunitのクレデンシャル
- **WHEN** GitHubのプロジェクトを担当する端末で `service install --user` を実行し、確認に `y` と答える
- **THEN** `token_ref` が `~/.config/credstore.encrypted/` に `--user` で暗号化されて置かれ、ユーザーunitのエージェントはそれを `$CREDENTIALS_DIRECTORY` から読む
