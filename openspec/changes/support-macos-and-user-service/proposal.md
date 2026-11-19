# Proposal

## Why

端末は Linux と systemd に固定されている。常駐はシステムサービスだけで sudo が要り、秘密は Secret Service が前提である。macOS では常駐も秘密の受け渡しも使えず、実行の sandbox の確認は何も調べずに通る。Secret Service の無いヘッドレスの Linux では、トークンの置き場所が開発用の `secrets.allow_env` しか無い。

systemd 256 以上では、ユーザーの範囲で暗号化したクレデンシャルをユーザーの unit で読める。ADR-032 がシステムサービスを選んだ理由（systemd 255 の制約）は、Ubuntu 26.04（systemd 259）には当てはまらなくなった。

## What Changes

- `switchboard service install|uninstall|status --user` を加える。sudo を使わずに、systemd のユーザー unit として常駐させる。linger を有効にし、systemd 256 未満では断る。既定は今のシステムサービスのまま。
- macOS では、`service install` が `~/Library/LaunchAgents/` に LaunchAgent を置き、`launchctl` で読み込む。クレデンシャルは渡さず、エージェントが実行時にログインキーチェーンからトークンを読む。
- sudo を使わない手順も、今の sudo の手順と同じく、全部を表示して確認を取ってから実行する（`--dry-run` と `--yes` も同じ扱い）。
- 秘密の保管先に、ユーザーの範囲の暗号化クレデンシャル（`~/.config/credstore.encrypted/`）を加える。Secret Service に接続できない Linux では、CLI もサービスもここを読み書きする。
- トークンを解決する順序を、`$CREDENTIALS_DIRECTORY`、OS の保管先（Secret Service／キーチェーン）、ユーザーの範囲の暗号化クレデンシャル、`allow_env` の環境変数とする。
- トークンが見つからないときの案内を、OS と保管先ごとに分ける。
- 実行の sandbox の事前の確認を、macOS では `sandbox-exec` で行う。Linux と macOS 以外の OS では、実行を誤りにする。
- sandbox の中から読めなくするパスに、`~/.config/credstore.encrypted` と `~/Library/Keychains` を加える。
- macOS は、オーナーが Mac の実機で Seatbelt を確かめるまで「未確認」と表示する。

## Capabilities

### New Capabilities

- `secrets`: Secret Service の無い端末の秘密の保管先（ユーザーの範囲の暗号化クレデンシャル）、そこへの書き込み、トークンが見つからないときの案内。

### Modified Capabilities

- `service`: systemd のユーザー unit と macOS の LaunchAgent の設置を加え、確認の流れとクレデンシャルの要件を、sudo を使わない手順に広げる。
- `config`: 「トークンの取り出し」の順序に、OS の保管先とユーザーの範囲の暗号化クレデンシャルを加える。
- `agent-run`: sandbox の事前の確認を macOS に広げ、ほかの OS を誤りにする。読めなくするパスを加える。

## Impact

- コード：`internal/service`（ユーザー unit と plist の生成、手順）、`cmd/switchboard/service.go`（`--user` と OS の分岐）、`internal/secret`（保管先と解決の順序、案内文）、`cmd/switchboard/relay.go` などの書き込み、`internal/claudecode`（`CheckSandbox`、読めなくするパス）。
- 外部のコマンド：`systemctl --user`、`loginctl enable-linger`、`systemd-creds --user`、`launchctl`、`/usr/bin/sandbox-exec`、`/usr/bin/security`（go-keyring 経由）。
- 文書：ADR-029 と ADR-032 への追記、新しい ADR、`docs/ROADMAP.md` の V 項目と常駐の行、`docs/GUIDE.md`、`docs/SECURITY.md`。
- 依存：変更 `own-run-sandbox` が加えた agent-run の要件（「sandboxを使えない端末」「sandboxの中から読めないパス」）を書き換える。この変更は、`own-run-sandbox` を archive したあとに archive する。
- 実装は Mac で別に行う。このブランチでは仕様の作成までにする。
