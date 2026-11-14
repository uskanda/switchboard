# Spec Delta

## MODIFIED Requirements

### Requirement: sandboxの中から読めないパス
エージェントは、sandboxの中のコマンドに、資格情報の置き場所を読ませてはならない（MUST NOT）。
資格情報の置き場所は、次のとおりとする。

- `~/.ssh`、`~/.gnupg`、`~/.git-credentials`、`~/.netrc`
- `~/.config/gh`、`~/.config/glab-cli`
- `~/.aws`、`~/.kube`、`~/.docker/config.json`
- `~/.config/credstore.encrypted`
- `~/Library/Keychains`

エージェントは、sandboxの中のコマンドに、switchboardの設定ファイルのディレクトリを読ませてはならない（MUST NOT）。
エージェントは、sandboxの中のコマンドに、状態のディレクトリのうちworktreeの置き場所以外を読ませてはならない（MUST NOT）。
エージェントは、端末に無いパスを、読めないパスとしてsandboxに渡してはならない（MUST NOT）。

#### Scenario: SSHの鍵を読む
- **WHEN** 実装の実行の中で、Claudeが `cat ~/.ssh/id_ed25519` を実行する
- **THEN** コマンドは失敗し、鍵の内容は出力に現れない

#### Scenario: ほかの実行の記録を読む
- **WHEN** 実装の実行の中で、Claudeが状態のディレクトリの `runs/` の下のファイルを `cat` で読む
- **THEN** コマンドは失敗する

#### Scenario: 端末に無い置き場所
- **WHEN** `~/.aws` が無い端末で、実装の実行が終わる
- **THEN** `~/.aws` は作られていない

#### Scenario: ユーザーの範囲のクレデンシャルを読む
- **WHEN** `~/.config/credstore.encrypted/switchboard_github` がある端末の実装の実行の中で、Claudeが `systemd-creds decrypt --user ~/.config/credstore.encrypted/switchboard_github -` を実行する
- **THEN** コマンドは失敗し、トークンの値は出力に現れない

#### Scenario: Macのキーチェーンを読む
- **WHEN** macOSの端末の実装の実行の中で、Claudeが `security find-generic-password -s switchboard/github -w` を実行する
- **THEN** コマンドは失敗し、トークンの値は出力に現れない

### Requirement: sandboxを使えない端末
sandboxを使えないとき、エージェントは、sandboxの無い `claude -p` で実行を進めてはならない（MUST NOT）。
Linuxで実行を始めるとき、エージェントは、`claude -p` を起動する前に、bwrapが名前空間を作れるかを確かめなければならない（MUST）。
macOSで実行を始めるとき、エージェントは、`claude -p` を起動する前に、`/usr/bin/sandbox-exec` で最小のプロファイルを動かせるかを確かめなければならない（MUST）。
確かめるのに失敗したとき、エージェントは、`claude -p` を起動せず、実行を誤りとして終えなければならない（MUST）。その理由は、issueのコメントに書く。
LinuxとmacOS以外のOSで実行を始めるとき、エージェントは、`claude -p` を起動せず、実行を誤りとして終えなければならない（MUST）。その理由は、issueのコメントに書く。

#### Scenario: bwrapの無い端末
- **WHEN** bwrapの入っていないLinuxの端末で、実装の実行を始める
- **THEN** `claude -p` は起動せず、実行は誤りになり、コメントはbwrapを使えないことを理由に挙げる

#### Scenario: 別のsandboxの中で動くエージェント
- **WHEN** 名前空間を作れない環境で動くエージェントが、実装の実行を始める
- **THEN** `claude -p` は起動せず、実行は誤りになり、コメントは名前空間を作れないことを理由に挙げる

#### Scenario: Seatbeltの中で動くエージェント
- **WHEN** すでに `sandbox-exec` の中で動いていて入れ子のプロファイルを適用できないmacOSのエージェントが、実装の実行を始める
- **THEN** `claude -p` は起動せず、実行は誤りになり、コメントはSeatbeltを使えないことを理由に挙げる

#### Scenario: 対応していないOS
- **WHEN** FreeBSDの端末で、実装の実行を始める
- **THEN** `claude -p` は起動せず、実行は誤りになり、コメントはこのOSでsandboxを確かめられないことを理由に挙げる
