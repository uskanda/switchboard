# Design

## Context

動機は proposal.md の Why を参照する。今の実装の要点は次のとおりである。

- `internal/service` は、unit を `/etc/systemd/system/` に、クレデンシャルを `/etc/credstore.encrypted/` に置く手順（`Step`）の一覧を作る。`cmd/switchboard/service.go` が、その一覧を表示し、確認を取ってから、手順ごとに sudo で実行する（ADR-032、ADR-034）。
- `internal/secret` の `Resolver` は、`$CREDENTIALS_DIRECTORY`、Secret Service（go-keyring）、`allow_env` の環境変数の順にトークンを探す。見つからないときの案内は `secret-tool store` だけである。
- `claudecode.CheckSandbox` は、Linux では bwrap を試し、それ以外の OS では何も調べずに通る。
- 受信箱のクライアントは、`/run/switchboard/inbox.sock` が無ければ `$XDG_RUNTIME_DIR/switchboard/inbox.sock` を探す。ユーザー unit の `RuntimeDirectory=switchboard` は後者に当たるので、クライアントは変えなくてよい。
- agent-run の「sandboxの中から読めないパス」と「sandboxを使えない端末」は、まだ archive していない変更 `own-run-sandbox` が加えた要件である。

## Goals / Non-Goals

**Goals:**

- sudo の要らない常駐を、Linux（systemd 256 以上のユーザー unit）と macOS（LaunchAgent）に用意する。
- Secret Service の無い Linux でも、トークンを平文で置かずに CLI とサービスが使えるようにする。
- macOS で、実行の sandbox を使えない環境を、実行の前に検出する。

**Non-Goals:**

- ログインしていない Mac での常駐（LaunchDaemon）。
- systemd 255 以下のヘッドレスの Linux に、`allow_env` 以外の置き場所を用意すること。
- Linux と macOS 以外の OS で動かすこと。対応しないことを明示して断るだけにする。
- このブランチでのコードの変更。実装は、オーナーが Mac で別に行う。

## Decisions

### 1. ユーザー unit は `--user` で選ぶ追加の道とする

ADR-032 がシステムサービスを選んだ理由は、systemd 255 ではユーザーの unit が暗号化したクレデンシャルを読めないことだった。systemd 256 で `systemd-creds encrypt --user` と、ユーザーの unit の `LoadCredentialEncrypted=` が加わった。そこで `--user` を加え、システムサービスと並べて選べるようにする。

- unit の中身は、システムサービスと同じ生成の関数から作り、`User=` を外し、`WantedBy=default.target` にする。`LoadCredentialEncrypted=` の参照先は `%h/.config/credstore.encrypted/<name>` とする。
- 手順は、`systemd-creds encrypt --user`、unit の書き込み、`systemctl --user daemon-reload`、`systemctl --user enable --now`、`loginctl enable-linger <user>` とする。sudo を付けないだけで、今の `Step` の表示と確認をそのまま使う。
- systemd の版は、`systemctl --version` の最初の行から読む。256 より古ければ、手順を作る前に断る。
- `service uninstall --user` は linger を無効にしない。利用者がほかの用途で linger を使っているかもしれないからである。

代わりの案：systemd 256 以上ではユーザー unit を既定にする（Q6 の B）。今のシステムサービスの利用者の操作が変わるので、採らない。

### 2. ユーザーの範囲の暗号化クレデンシャルを、秘密の保管先にする

`systemd-creds encrypt --user` の鍵は、root だけが読めるホストの鍵と、UID と、machine-id から作られる。別のユーザーやほかの端末では復号できない。同じファイルを、CLI は `systemd-creds decrypt --user` で読み、ユーザー unit は `LoadCredentialEncrypted=` で読む。そのため、トークンを複製せずに済む。

- ファイル名は、`$CREDENTIALS_DIRECTORY` と同じ `secret.CredentialName(ref)` とする。ユーザー unit のクレデンシャルとファイル名が一致するので、`service install --user` は、ファイルがすでにあれば、Secret Service から読まずにそのまま使う。
- 「Secret Service に接続できない」とは、D-Bus のセッションバスが無いか、`org.freedesktop.secrets` を提供するものがいないことを指す。項目が無いだけのときや、接続したうえでの誤りは含めない。接続したうえでの誤りは、今までどおり報告する。
- 解決の順序は、`$CREDENTIALS_DIRECTORY`、OS の保管先、ユーザーの範囲の暗号化クレデンシャル（Linux で Secret Service に接続できないときだけ）、`allow_env` の環境変数とする。
- 書き込みは、`Store` の `Set` を、保管先を選ぶ層の後ろに隠す。値は `systemd-creds encrypt --user --name=<name> - <path>` の標準入力で渡し、引数に出さない。

代わりの案：0600 の平文ファイル（Q5 の D）。sandbox の外の同じユーザーのプロセスが読めてしまうので、採らない。

### 3. macOS は LaunchAgent と、ログインキーチェーンからの読み取り

- plist は `~/Library/LaunchAgents/<label>.plist` に置く。ラベルは unit の名前に合わせ、`dev.switchboard.agent`、`dev.switchboard.relay`、`dev.switchboard.inbox`、`dev.switchboard.web` とする。`ProgramArguments` に `switchboard agent run --config <絶対パス>` を、`KeepAlive` に `SuccessfulExit=false` を、`RunAtLoad` に true を書く。ログは `StandardOutPath` と `StandardErrorPath` で、状態のディレクトリの下に出す。
- 手順は、plist の書き込み、`launchctl bootout gui/<uid>/<label>`（設置済みのときだけ）、`launchctl bootstrap gui/<uid> <plist>` とする。
- go-keyring は、macOS では `/usr/bin/security` を呼んでキーチェーンを読み書きする。項目は、サービス名を `token_ref`、アカウント名を `token` にした汎用パスワードで、Linux の Secret Service の属性（service と username）に対応する。
- `service install` は、macOS ではトークンを読まず、クレデンシャルも作らない。ただし、対象の `token_ref` をキーチェーンから読めることは、設置の前に `Resolver` で確かめ、読めないものがあれば何もせずに断る。Linux の install がトークンを読めなければ暗号化の前に止まるのと、結果を揃える。

代わりの案：LaunchDaemon（Q2 の B）。sudo が要り、ログインキーチェーンが解錠されないので、トークンを別に置く必要がある。採らない。

### 4. macOS の sandbox の事前の確認

`CheckSandbox` は、macOS では `/usr/bin/sandbox-exec -p '(version 1)(allow default)' /usr/bin/true` を動かす。入れ子の Seatbelt の中など、プロファイルを適用できない環境では失敗するので、実行を誤りにする。Linux と macOS 以外の OS では、何も調べずに通すのをやめ、誤りにする。

読めなくするパスに `~/Library/Keychains` を足しても、キーチェーンの読み取りは防げない。キーチェーンへのアクセスは、ファイルの読み取りではなく securityd への Mach IPC だからである。ファイルを直接読む道を塞ぐために足し、IPC の道は Seatbelt の mach-lookup の制限に頼る。それが働くかは V 項目で確かめる。

### 5. テストは OS に依存させない

`service` の手順と unit／plist の文字列は、OS を引数に取る純粋な関数で作り、Linux の CI で macOS の分も確かめる。`runtime.GOOS` を見るのは、`cmd/switchboard` の入口と `CheckSandbox` だけにする。`sandbox-exec` と `systemd-creds` は、今の偽の bwrap と同じく、偽の実行ファイルでテストする。

## Risks / Trade-offs

- [Seatbelt が `com.apple.*` の mach-lookup を広く許していて、sandbox の中から `security find-generic-password` でトークンを読める] → V 項目で確かめ、macOS を「確認済み」にする条件にする。読めた場合は、switchboard 専用のキーチェーンファイルにトークンを置き、エージェントだけが解錠する形に改める（Q3 の B）。そのときは grilling をやり直す。
- [ユーザー unit で `LoadCredentialEncrypted=` が `--user` のクレデンシャルを読めない] → tasks の最初に、この端末（systemd 259）で確かめる。動かなければ、そこで止めて報告する。
- [linger を有効にできない（ポリシーで自分のユーザーでも拒否される）] → `loginctl enable-linger` の失敗をそのまま報告し、unit は置かない。手順は表示済みなので、利用者が管理者に頼める。
- [`systemd-creds decrypt --user` の varlink ソケットに sandbox の中から届く] → bwrap の sandbox は unix ソケットの接続を制限している。V 項目で、sandbox の中から復号できないことを確かめる。
- [ログインしていない Mac ではエージェントが動かない] → 受け入れる。GUIDE に書く。
- [macOS の受信箱のソケット] → macOS には `XDG_RUNTIME_DIR` が無い。置き場所は Open Questions に残す。

## Migration Plan

- 今のシステムサービスの利用者には影響しない。`--user` を付けない限り、手順も置き場所も変わらない。
- システムサービスからユーザー unit に移るときは、`service uninstall` のあとに `service install --user` を実行する。両方が同時に動くと同じ issue を取り合うので、`service install --user` は、システムサービスの unit があれば断り、撤去を案内する（逆も同じ）。
- 戻すときは、`service uninstall --user` のあとに `service install` を実行する。

## Open Questions

- macOS の受信箱のソケットの置き場所（`$TMPDIR` か、状態のディレクトリの下か）。クライアントの探し方は `inboxapi.DefaultSocket` の中で閉じるので、仕様と手順は変わらない。Mac での実装のときに決める。
- LaunchAgent のラベルの接頭辞（`dev.switchboard.` か、リポジトリに合わせた別の名前か）。仕様には現れない。
