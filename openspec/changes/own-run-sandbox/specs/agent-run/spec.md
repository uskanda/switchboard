## ADDED Requirements

### Requirement: ユーザー設定の切り離し
エージェントは、`--setting-sources project,local` を付けて `claude -p` を起動しなければならない（MUST）。
エージェントは、ユーザーの全体設定のsandbox、フック、プラグイン、許可の規則を、実行に使わせてはならない（MUST NOT）。

#### Scenario: ユーザーの全体設定に許可とフックがある
- **WHEN** ユーザーの全体設定に `Bash(curl *)` の許可とPreToolUseのフックがある状態で、実装の実行の中でClaudeがBashを呼ぶ
- **THEN** フックは呼ばれず、`curl` の許可は実行の許可の規則に無い

### Requirement: 実行のsandbox
エージェントは、実行ごとの設定ファイルでsandboxを有効にして、`claude -p` を起動しなければならない（MUST）。
sandboxの中のコマンドに、エージェントは、worktreeと `$TMPDIR` への書き込みだけを許さなければならない（MUST）。
エージェントは、sandboxの中のコマンドに、ネットワークへの接続を許してはならない（MUST NOT）。
エージェントは、sandboxの外でのコマンドの再実行を受け付けてはならない（MUST NOT）。

#### Scenario: worktreeの外に書く
- **WHEN** 実装の実行の中で、Claudeが `touch ~/outside` を実行する
- **THEN** コマンドは失敗し、ファイルは作られない

#### Scenario: ネットワークに接続する
- **WHEN** 実装の実行の中で、Claudeが `curl https://example.com` を実行する
- **THEN** 接続は拒否され、コマンドは失敗する

#### Scenario: sandboxの外で再実行する
- **WHEN** 実装の実行の中で、ClaudeがBashを `dangerouslyDisableSandbox` 付きで呼ぶ
- **THEN** コマンドはsandboxの中で実行されるか拒否され、sandboxの外では実行されない

### Requirement: sandboxの外で動くコマンド
ファイルを書く実行では、エージェントは、プロジェクトの検証コマンドと `git` だけをsandboxの外で実行させなければならない（MUST）。
計画の実行では、エージェントは、`git` だけをsandboxの外で実行させなければならない（MUST）。
sandboxの外で動くコマンドを実行できるかは、許可の規則と拒否の規則で決まらなければならない（MUST）。

#### Scenario: 検証コマンドがunixソケットを使う
- **WHEN** `make verify` が検証コマンドのプロジェクトで、実装の実行の中でClaudeが `make verify` を実行し、検証がunixソケットを開くテストを含む
- **THEN** コマンドはsandboxの外で実行され、テストはソケットを開ける

#### Scenario: 検証コマンドでないmakeのターゲット
- **WHEN** `make verify` が検証コマンドのプロジェクトで、実装の実行の中でClaudeが `make deploy` を実行する
- **THEN** コマンドはsandboxの中で実行され、ネットワークに接続できない

#### Scenario: git push
- **WHEN** 実装の実行の中で、Claudeが `git push` を実行しようとする
- **THEN** コマンドは拒否され、resultの `permission_denials` に記録される

### Requirement: sandboxの中のコマンドの許可
エージェントは、sandboxの中で動くBashのコマンドを、許可の規則が無くても実行させなければならない（MUST）。
拒否の規則に当たるコマンドは、sandboxの中で動くものでも、拒否されなければならない（MUST）。

#### Scenario: 許可の規則の無いコマンド
- **WHEN** 実装の実行の中で、Claudeが `python3 -c 'print(1)'` を実行する
- **THEN** コマンドは実行され、resultの `permission_denials` に記録されない

#### Scenario: 拒否の規則に当たるコマンド
- **WHEN** 実装の実行の中で、Claudeが `gh issue list` を実行しようとする
- **THEN** コマンドは拒否され、resultの `permission_denials` に記録される

### Requirement: sandboxの中から読めないパス
エージェントは、sandboxの中のコマンドに、資格情報の置き場所を読ませてはならない（MUST NOT）。
資格情報の置き場所は、次のとおりとする。

- `~/.ssh`、`~/.gnupg`、`~/.git-credentials`、`~/.netrc`
- `~/.config/gh`、`~/.config/glab-cli`
- `~/.aws`、`~/.kube`、`~/.docker/config.json`

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

### Requirement: sandboxを使えない端末
sandboxを使えないとき、エージェントは、sandboxの無い `claude -p` で実行を進めてはならない（MUST NOT）。
Linuxで実行を始めるとき、エージェントは、`claude -p` を起動する前に、bwrapが名前空間を作れるかを確かめなければならない（MUST）。
bwrapが名前空間を作れないとき、エージェントは、`claude -p` を起動せず、実行を誤りとして終えなければならない（MUST）。その理由は、issueのコメントに書く。

#### Scenario: bwrapの無い端末
- **WHEN** bwrapの入っていないLinuxの端末で、実装の実行を始める
- **THEN** `claude -p` は起動せず、実行は誤りになり、コメントはbwrapを使えないことを理由に挙げる

#### Scenario: 別のsandboxの中で動くエージェント
- **WHEN** 名前空間を作れない環境で動くエージェントが、実装の実行を始める
- **THEN** `claude -p` は起動せず、実行は誤りになり、コメントは名前空間を作れないことを理由に挙げる

### Requirement: sandboxの中での作業の指示
エージェントは、追記する指示に、sandboxの中での作業の仕方を書かなければならない（MUST）。
指示には、ファイルの読み書きに `Read`、`Grep`、`Glob`、`Edit`、`Write` を使うことを含める。
ファイルを書く実行の指示には、検証コマンドと `git` をパイプや `&&` でつながずに単独で実行し、`cd` から始めないことを含める。

#### Scenario: 実装の実行の指示
- **WHEN** `make verify` が検証コマンドのプロジェクトで、実装の実行を始める
- **THEN** `--append-system-prompt-file` のファイルに、`make verify` を単独で実行するように書かれている

## MODIFIED Requirements

### Requirement: 追加の許可を使う実行
ファイルを書く実行では、エージェントは、`<agent.allow_tools_extra>` を許可するツールに加えなければならない（MUST）。
そのプロジェクトの `<projects[].allow_tools_extra>` も、同じように加える。
エージェントは、加えた各項目のツール名（`Bash(openspec *)` なら `Bash`）を、渡すツールにも加えなければならない（MUST）。
ファイルを書く実行とは、直接の実装、承認のあとの実装、CIとpushの失敗を直す実行、baseとの衝突を直す実行である。
エージェントは、計画の実行に、追加の許可を加えてはならない（MUST NOT）。
追加の許可があっても、組み込みで拒否している操作は、拒否したままでなければならない（MUST）。

#### Scenario: スキルを使う
- **WHEN** `<agent.allow_tools_extra>` に `Skill` がある状態で、承認のあとの実装の実行の中で、ClaudeがSkillツールを呼ぶ
- **THEN** 実行のinitの `tools` に `Skill` があり、呼び出しは許可され、resultの `permission_denials` に記録されない

#### Scenario: プロジェクトだけの許可
- **WHEN** `Skill` は `gh:uskanda/switchboard` の `<projects[].allow_tools_extra>` だけにある。別のプロジェクトの実装の実行を始める
- **THEN** 実行のinitの `tools` に `Skill` は無い

#### Scenario: 計画の実行
- **WHEN** `<agent.allow_tools_extra>` に `Skill` がある状態で、計画の実行を始める
- **THEN** 実行のinitの `tools` に `Skill` は無い
