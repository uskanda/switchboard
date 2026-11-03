# Design

## Context

`claude -p` は今、ユーザーの全体設定（`~/.claude/settings.json`）のsandboxと許可の規則を引き継いでいる。実行ごとの設定ファイルは許可と拒否の規則だけを持ち、`dontAsk` では規則に無いコマンドが拒否される。213回の実行のうち76回で拒否が起き、23回の実行は `dangerouslyDisableSandbox` で再実行を試みた。

実物での確認の結果（claude 2.1.316）は次のとおり。

- `--setting-sources project,local` と、`--settings` に書いたsandboxを組み合わせても、sandboxは働く。worktreeの外への書き込みは `Read-only file system` で失敗し、ドメインの許可が無いとネットワークへの接続はプロキシに拒否される。
- sandboxの中では、unixソケットを開けない。`excludedCommands` に入れたコマンドはsandboxの外で動き、inboxserveのテストが通る。sandboxの外で動くコマンドにも、許可の規則が要る。
- sandboxが有効だと、sandboxの中のBashのコマンドは、許可の規則が無くても実行される（`autoAllowBashIfSandboxed`、既定で有効）。拒否の規則は働く。この設定を切ると、パイプや `&&` でつないだコマンドは、規則の無い部分が1つあれば拒否される。
- `failIfUnavailable: true` では、bwrapとsocatが無いとclaudeは起動を拒む。bwrapはあるが名前空間を作れない環境では、claudeは起動し、Bashがすべて `bwrap: Creating new namespace failed` で失敗する。
- `bwrap --ro-bind / / --dev /dev --unshare-all true` は、名前空間を作れない環境で同じ誤りを出して終了コード1になり、作れる環境では0になる。

## Goals / Non-Goals

**Goals:**

- 実行のsandboxと許可の決め方を、switchboardの設定ファイルだけで決める。
- 許可の規則が無いための拒否と、sandboxの外での再実行を無くす。
- sandboxを使えない端末で、sandboxの無い実行を進めない。

**Non-Goals:**

- プロジェクトごとに、sandboxのネットワークと書き込みの許可を足す設定項目。
- macOSでの事前の確認。macOSのsandboxはOSに組み込まれている。

## Decisions

### 設定ファイルにsandboxを書く

`claudecode.Settings` に、sandboxの外で動かすコマンドと、読めなくするパスを足す。`Run` は設定ファイルに次を書く。

```json
{
  "permissions": {"allow": [...], "deny": [...]},
  "sandbox": {
    "enabled": true,
    "failIfUnavailable": true,
    "allowUnsandboxedCommands": false,
    "autoAllowBashIfSandboxed": true,
    "excludedCommands": ["make verify", "git", "git *"],
    "filesystem": {"denyRead": ["~/.ssh", "..."]}
  }
}
```

書き込みの許可とネットワークの許可は書かない。CLIの既定は、worktreeと `$TMPDIR` への書き込みだけを許し、ドメインを許さない。`autoAllowBashIfSandboxed` は既定と同じ値だが、明示する。CLIの既定が変わっても、許可の決め方を変えないためである。`Args` は `--setting-sources project,local` を付ける。

### sandboxの外で動かすコマンド

ファイルを書く実行では、プロジェクトの検証コマンドの全体と `git` を、`excludedCommands` に入れる。検証コマンドの先頭の語（`make`）にすると、`make deploy` もsandboxの外で動いてしまう。検証コマンドの全体を入れれば、ほかのターゲットはsandboxの中で動く。許可の規則は今までどおり、検証コマンドの先頭の語で作る。計画の実行では、`git` だけを入れる。計画の実行は `make` を拒否しているためである。

`git` をsandboxの外で動かす理由は、worktreeの `.git` がクローンの `.git/worktrees/` を指し、そこはworktreeの外にあるためである。sandboxの中では `git commit` が書き込めない。`git push` と `git rebase` は、拒否の規則で止める。

### 読めなくするパスは、実行の前に端末にあるものだけにする

V46によると、sandboxは書き込みを拒否するパスに0バイトのファイルを置き、強制終了のあとはそのファイルが残る。読み込みを拒否するパスにも同じことが起きると、`~/.aws` という空のファイルが残り、AWSのCLIは設定を読めなくなる。そこで、エージェントは実行の前にパスがあるかを確かめ、あるものだけを `denyRead` に入れる。

読めなくするパスは次の3つからなる。

- 資格情報の固定の一覧（specsの「sandboxの中から読めないパス」）。
- switchboardの設定ファイルのディレクトリ。`Agent` に、読み込んだ設定ファイルのパスを持たせる。
- 状態のディレクトリの直下の項目のうち、`worktrees` 以外のもの。worktreeは状態のディレクトリの下にあるので、ディレクトリ全体は拒否できない。

ほかのプロジェクトのworktreeは読める。中身はプロジェクトのコードで、利用者の端末の `~/repos` などにも同じものがある。

### bwrapの確認はアダプタに置く

`claudecode` に、sandboxを使えるかを確かめる関数を足す。Linuxでは、`bwrap --ro-bind / / --dev /dev --unshare-all true` を数秒の期限で実行し、終了コードと標準エラーを返す。Linux以外では何もしない。エージェントは実行ごとに、claudeを起動する前に呼ぶ。失敗したら、`o.fail` で実行を誤りにし、bwrapの標準エラーを理由としてコメントに書く。確認はミリ秒で終わるので、起動時に1回だけにする理由は無い。実行ごとに確かめれば、エージェントの起動のあとで環境が変わっても検出できる。

### システムプロンプトの指示

`prompt.System` と `prompt.PlanSystem` に、sandboxの節を足す。書く内容は次のとおり。

- コマンドはsandboxの中で動く。書き込めるのはカレントディレクトリと `$TMPDIR` だけで、ネットワークには接続できない。
- ファイルの読み書きと検索は、`Read`、`Grep`、`Glob`、`Edit`、`Write` を使い、`cat >`、ヒアドキュメント、`python3` で書かない。
- ファイルを書く実行では、検証コマンドと `git` を単独のコマンドとして実行する。パイプや `&&` でつなげず、`cd` から始めない。sandboxの外で動くのは、単独のコマンドだけである。

計画の実行の「Other commands are refused」は、sandboxの中のコマンドが動くことに合わせて書き直す。

### 追加の許可の意味の変化

sandboxの中のBashのコマンドは規則なしで動くため、`allow_tools_extra` の `Bash(openspec *)` のような規則は、ほとんど意味を持たなくなる。`Skill` のような、Bash以外のツールを足す役割は残る。設定の項目は変えず、READMEの説明を直す。

## Risks / Trade-offs

- リスク：sandboxの中のコマンドが、一覧に無い資格情報のファイルを読み、worktreeに書いてcommitする。対策：ネットワークは使えないので、外に出る経路はswitchboardのpushだけになる。フォージのトークンはSecret Serviceにあり、sandboxの中からはD-Busのunixソケットを使えないため読めない。claudeに渡す環境変数は、許可した変数だけである（ADR-012）。
- リスク：sandboxの中では依存の取得ができず、`go mod download` や `pnpm install` が失敗する。対策：検証コマンドはsandboxの外で動く。必要になったら、プロジェクトごとの設定項目を足す。
- リスク：`excludedCommands` が検証コマンドの全体を、前方一致でなく完全一致で照合するかは、まだ確かめていない。対策：実装の前に実物で確かめ、ROADMAPの検証項目に書く。
- リスク：利用者の全体設定にある許可の規則が、実行に使われなくなる。対策：READMEに、必要な許可を `allow_tools_extra` に書くよう記す。

## Migration Plan

更新したバイナリで再起動したあとの実行から適用される。bwrapとsocatの無いLinuxの端末では、実行が誤りになる。その端末では、bwrapとsocatを入れてから再起動する。戻すときは、前のバイナリに戻す。
