# grilling 記録: support-macos-and-user-service

issue `gh:uskanda/switchboard` #147 で、Q1〜Q6 の質問と推奨回答を出した。オーナーは各問いに答え、計画を承認した。この記録は、その質問と返答をまとめたものである。

## 決定

| 決定 | 選択 | 出典 |
|---|---|---|
| この変更で扱う範囲 | A：1つの OpenSpec の変更で、macOS と Linux の両方を扱う。推奨は B（Linux だけ実装し、macOS は方針だけ決める）だったが、オーナーは A を選んだ。実装は Mac で別に行うため、このブランチでは仕様の作成までにする | issue #147 Q1 と、オーナーの補足 |
| macOS の常駐の形 | A：`~/Library/LaunchAgents/` に置く LaunchAgent（ユーザーのセッションの gui ドメイン）。sudo は要らない。ログインしていないと動かない。ADR-034 の「表示して確認してから実行する」流れは、sudo が要らなくてもそのまま使う | issue #147 Q2 |
| macOS のトークンの渡し方 | A：常駐のエージェントが、実行時にログインキーチェーンから go-keyring で読む。実行の sandbox の中から `security find-generic-password` で読めないことを、確かめる条件にする。読めると分かったら、専用のキーチェーンに改める | issue #147 Q3 |
| macOS の sandbox（Seatbelt）を確かめる方法 | A：オーナーが Mac の実機で V 項目を確かめる。結果が出るまで、macOS は「未確認」と表示する。あわせて、`CheckSandbox` は macOS で `/usr/bin/sandbox-exec` を使って最小のプロファイルを動かし、失敗したら実行を誤りにする（「はい」） | issue #147 Q4 |
| Secret Service の無い Linux のトークンの置き場所 | B：ユーザーの範囲の暗号化クレデンシャル（`systemd-creds encrypt --user`、`~/.config/credstore.encrypted/`）を秘密の保管先に加える。CLI もサービスも読み書きする。systemd 256 以上が要る。Ubuntu 24.04（systemd 255）のヘッドレス端末は、上げるまで `allow_env` のままとする | issue #147 Q5 |
| sudo の要らない systemd のユーザー unit | A：`service install --user` を加える。unit は `~/.config/systemd/user/` に置き、Q5 の置き場所から `LoadCredentialEncrypted=` で読む。linger を有効にし、systemd 256 未満では断る。既定は今のシステムサービスのまま。ADR-032 には、systemd 256 以上ではユーザー unit も選べると追記する | issue #147 Q6 |
| 計画 | issue #147 の計画の承認のお願い（comment 6659685909）を、オーナーが `/ok` で承認した | 会話中に確認 |

## 後回しにしたもの

- コードの変更、ADR の追記と新しい ADR、ROADMAP の V 項目の追加。オーナーが Mac で別に行う（tasks.md に載せる）。
- Ubuntu 24.04（systemd 255）のヘッドレス端末に、`service install` が端末からトークンを入力させる道（Q5 の A）。足りなければ、あとで C（A と B の両方）に広げる。
- macOS のログインしていない間の常駐（LaunchDaemon、Q2 の B）。
- Linux と macOS 以外の OS への対応。

## 状態

frontier は空。共有理解は 2026-11-05 に確認済み。
