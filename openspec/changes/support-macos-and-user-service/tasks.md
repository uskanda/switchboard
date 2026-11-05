# Tasks

実装はオーナーが Mac で別に行う。issue #147 のブランチでは、仕様の作成（grilling.md、proposal.md、specs、design.md、tasks.md）までを済ませた。

## 1. 実物での確認（V 項目）

- [ ] 1.1 Linux（systemd 256 以上、この端末は systemd 259）で、`systemd-creds encrypt --user` で暗号化したファイルを、ユーザー unit が `LoadCredentialEncrypted=` で読めることを、試しの unit で確かめる。linger の下で、ログアウトと再起動の後も unit が動くことも確かめる。どちらかが成り立たなければ、ここで止めて issue に報告する
- [ ] 1.2 Linux の実行の sandbox の中から、`systemd-creds decrypt --user` でユーザーの範囲のクレデンシャルを復号できないことを、実物の `claude` で確かめる
- [ ] 1.3 Mac で、Claude Code の sandbox（Seatbelt）が ADR-057 の要件を満たすことを確かめる。`failIfUnavailable` が働くこと、worktree と `$TMPDIR` の外に書けないこと、ネットワークに出られないこと、denyRead のパスを読めないことを見る
- [ ] 1.4 Mac で、実行の sandbox の中から `security find-generic-password -s <ref> -w` でトークンを読めないことを確かめる。読めたら、ここで止めて issue に報告する（design.md の Risks のとおり、専用のキーチェーンに改めるかを決め直す）
- [ ] 1.5 Mac で、`sandbox-exec` の中から入れ子の `sandbox-exec` が失敗することを確かめる
- [ ] 1.6 1.1〜1.5 の結果を、`docs/ROADMAP.md` の検証項目に、次の番号の V 項目として書く。結果が出るまで、macOS の行は「未確認」とする

## 2. 秘密の保管先と解決の順序

- [ ] 2.1 `internal/secret` に、ユーザーの範囲の暗号化クレデンシャルの保管先を加える。読み書きは偽の `systemd-creds` でテストし、値が引数に出ないことを確かめる
- [ ] 2.2 Secret Service に接続できないことと、項目が無いことと、接続したうえでの誤りを区別し、解決の順序を `$CREDENTIALS_DIRECTORY`、OS の保管先、ユーザーの範囲の暗号化クレデンシャル、`allow_env` にする。順序と、保管先の誤りを環境変数で置き換えないことを、テストを先に書いて確かめる
- [ ] 2.3 systemd 256 未満で Secret Service に接続できないとき、読み書きのコマンドが案内して失敗することを、テストで確かめる
- [ ] 2.4 見つからないときの案内を、Secret Service、ユーザーの範囲の暗号化クレデンシャル、macOS のキーチェーンで分け、それぞれの文をテストで確かめる
- [ ] 2.5 `relay token add` などの書き込みを、保管先を選ぶ層を通すように直し、Secret Service の無い場合に暗号化クレデンシャルへ書くことをテストで確かめる
- [ ] 2.6 `docs/GUIDE.md` と `docs/SECURITY.md` に、Secret Service の無い Linux でのトークンの置き方と、鍵が UID と端末に結び付くことを書く

## 3. systemd のユーザー unit

- [ ] 3.1 `internal/service` に、ユーザー unit の生成（`User=` を外し、`WantedBy=default.target`、`LoadCredentialEncrypted=` を `%h/.config/credstore.encrypted/` から読む）を加え、unit の文字列をテストで確かめる
- [ ] 3.2 ユーザー unit の手順（`systemd-creds encrypt --user`、`systemctl --user`、`loginctl enable-linger`）を、sudo を付けない `Step` として作る。エージェント、中継、受信箱、WUI のそれぞれの手順の一覧をテストで確かめる
- [ ] 3.3 `cmd/switchboard/service.go` に `--user` を加える。systemd 256 未満と、同じ名前のシステムサービスとユーザー unit の取り違えを断ること、表示と確認と `--dry-run`／`--yes` が今の流れと同じになることをテストで確かめる
- [ ] 3.4 `service status --user` と `service uninstall --user` をテストで確かめる。uninstall が linger を無効にしないことも確かめる
- [ ] 3.5 1.1 の端末で、`service install --user` から `service status --user`、`service uninstall --user` までを実物で通す
- [ ] 3.6 `docs/DECISIONS.md` の ADR-032 に、systemd 256 以上ではユーザー unit も選べると追記し、ユーザー unit とユーザーの範囲の保管先の新しい ADR を足す。`docs/GUIDE.md` に `--user` の使い方と linger を書く

## 4. macOS の LaunchAgent

- [ ] 4.1 `internal/service` に、OS を引数に取る plist の生成を加える。エージェント、中継、受信箱、WUI の plist の文字列をテストで確かめる
- [ ] 4.2 macOS の手順（plist の書き込み、設置済みなら `launchctl bootout`、`launchctl bootstrap gui/<uid>`）と、撤去と状態の表示の手順を作り、Linux の上のテストで確かめる
- [ ] 4.3 macOS では設置の前に対象の `token_ref` をキーチェーンから読めるかを確かめ、読めなければ何もしないことをテストで確かめる
- [ ] 4.4 Linux と macOS 以外の OS で `service` の各コマンドが断ることをテストで確かめる
- [ ] 4.5 受信箱のソケットの macOS での置き場所を決め（design.md の Open Questions）、`inboxapi` のテストで確かめる
- [ ] 4.6 Mac の実機で、`service install` から `service status`、`service uninstall` までを通し、ログイン後にエージェントが動くことを確かめる
- [ ] 4.7 ADR-029 に macOS への対応を追記し、macOS の常駐とトークンの新しい ADR を足す。`docs/GUIDE.md` に Mac での設置とキーチェーンへのトークンの置き方を書く

## 5. 実行の sandbox

- [ ] 5.1 `claudecode.CheckSandbox` に、macOS で `/usr/bin/sandbox-exec` の最小のプロファイルを動かす分岐を加える。偽の `sandbox-exec` で、成功と失敗をテストで確かめる
- [ ] 5.2 Linux と macOS 以外の OS で `CheckSandbox` が誤りを返し、実行が誤りになって理由がコメントに書かれることをテストで確かめる
- [ ] 5.3 読めなくするパスに `~/.config/credstore.encrypted` と `~/Library/Keychains` を加え、端末に無いものを除くことをテストで確かめる
- [ ] 5.4 `docs/SECURITY.md` に、キーチェーンへのアクセスは Mach IPC で、denyRead だけでは防げないことと、1.4 の結果を書く

## 6. 全体の確認

- [ ] 6.1 `docs/ROADMAP.md` の常駐の行を、ユーザー unit と macOS に合わせて直し、1.6 の V 項目の結果で macOS の「未確認」を外せるかを判断する
- [ ] 6.2 `make verify` が通ることを確かめる
- [ ] 6.3 `own-run-sandbox` を archive したあとに、`openspec validate support-macos-and-user-service --strict` が通ることを確かめる
