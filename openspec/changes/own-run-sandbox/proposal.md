# Proposal

## Why

`claude -p` は、ユーザーの全体設定のsandboxと許可の規則をそのまま引き継いでいる。213回の実行のうち76回で、許可の規則が無いためにコマンドが拒否された（計168回）。23回の実行は `dangerouslyDisableSandbox` でsandboxの外での再実行を試みた。sandboxの中身と許可の決め方をswitchboardが持ち、拒否と再実行で増えるトークンと処理時間を減らす（#130）。

## What Changes

- `claude -p` に `--setting-sources project,local` を付け、ユーザーのsandbox、フック、プラグイン、許可の規則を実行から外す。
- 実行ごとの設定ファイルで、sandboxを有効にする。書き込みはworktreeと `$TMPDIR` だけに許し、ネットワークは許さない。sandboxの外での再実行は受け付けない。
- sandboxの外で動かすのは、検証コマンドと `git` だけにする。この2つは、今までどおり許可の規則で決める。
- sandboxの中のBashのコマンドは、許可の規則が無くても実行させる。拒否の規則は残す。
- sandboxの中から、資格情報の置き場所と、switchboardの設定と状態のディレクトリを読めなくする。
- sandboxを使えない端末では、claudeを起動せず、実行を誤りにする。Linuxでは、実行の前にエージェントがbwrapを1回試す。
- システムプロンプトに、sandboxの中での作業の仕方を書く。

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

- `agent-run`: 起動の設定の切り離し、sandboxの中身、sandboxを使えないときの扱い、システムプロンプトの指示を足す。
- `plan-mode`: 計画の実行の権限を、sandboxの自動の許可に合わせて書き直す。

## Impact

- コード：`internal/claudecode` は、起動の引数と設定ファイルのsandboxとbwrapの確認を持つ。`internal/agent` は実行ごとのsandboxの中身を決め、`internal/prompt` はシステムプロンプトを持つ。
- 文書：`docs/DECISIONS.md` にADRを足す。`docs/ROADMAP.md` の検証項目に、sandboxの確認結果を書く。`docs/SECURITY.md`、`docs/DESIGN.md`、`README.md` の許可とsandboxの説明を直す。
- 運用：ユーザーの全体設定のsandboxと許可の規則は、実行に使われなくなる。必要な許可は `allow_tools_extra` に書く。bwrapとsocatの無いLinuxの端末では、実行が誤りになる。
