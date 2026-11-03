# grilling 記録: own-run-sandbox

## 決定

| 決定 | 選択 | 出典 |
|---|---|---|
| sandboxを誰が決めるか | switchboardの `--settings` がsandboxを有効にし、中身を決める。`dangerouslyDisableSandbox` による再実行は受け付けない | 第1ラウンド Q1 |
| ユーザー設定の切り離し | `--setting-sources project,local` を付け、ユーザーのsandbox、フック、プラグイン、許可の規則を外す | 第1ラウンド Q2 |
| 読み取りだけのコマンドの許可 | 一覧を足す案だったが、第3ラウンド Q7で不要になった | 第1ラウンド Q3 |
| システムプロンプトの指示 | ファイルの読み書きはRead、Grep、Glob、Edit、Writeを使う。sandboxの外で動くコマンドは、パイプや `&&` でつながず、`cd` から始めない | 第1ラウンド Q4 |
| sandboxの中身 | 書き込みはworktreeと `$TMPDIR` だけ。許可するドメインは無し。sandboxの外で動かすのは検証コマンドと `git` | 第2ラウンド Q5 |
| bwrapが入っていない端末 | `failIfUnavailable: true` にし、claudeの起動を失敗させる | 第2ラウンド Q6 |
| Bashの許可の決め方 | sandboxの自動の許可を使い、sandboxをBashの境界にする。sandboxの外で動くコマンドだけを許可の規則で決める | 第3ラウンド Q7 |
| 入れ子のbwrapの失敗 | Linuxでは、エージェントが実行の前にbwrapを1回試す。失敗したらclaudeを起動せず、実行を誤りにする | 第3ラウンド Q8 |
| sandboxの中から読めなくするパス | 資格情報の固定の一覧と、switchboardの設定と状態のディレクトリ | 第4ラウンド Q9 |
| sandboxの設定項目 | 今は作らない | 第4ラウンド Q10 |

## 後回しにしたもの

- プロジェクトごとに、sandboxのネットワークと書き込みの許可を足す設定項目。
- 同じコマンドで最初の要求の大きさが約7kと約18万トークンに分かれた現象。実際の実行ログには出ていない。

## 状態

frontierは空。共有理解は2026-11-03に確認済み。
