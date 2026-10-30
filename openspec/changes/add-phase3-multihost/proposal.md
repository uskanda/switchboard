# Proposal

## Why

Phase 2までで、1台の端末がGitHubとGitLabのプロジェクトを扱えるようになった。しかし、エージェントは、ログインしたシェルから手で起動するしかない。端末を再起動したり、SSHを切ったりすると止まる。また、設定の誤りで2台が同じプロジェクトを担当すると、同じissueを2台が同時に実行しうる。ROADMAP.mdのPhase 3の完了条件は、2台以上の端末で、それぞれの担当issueだけが処理されることである。

## What Changes

- 着手のときに、エージェントはマーカー付きの着手のコメントを投稿してからissueを取り直す。最も早く着手した端末だけが実行し、他の端末は自分の着手のコメントを消して退く
- `switchboard service install`、`service uninstall`、`service status` を足す。Linuxでは、`User=` を指定したsystemdのシステムサービスとしてエージェントを常駐させる
- `service install` は、この端末のエージェント用のトークンを、ホストの鍵で暗号化したsystemdのクレデンシャルにして置く。sudoで実行するコマンドを、事前に全部表示して確認を取る
- トークンの取り出しに、systemdのクレデンシャル（`$CREDENTIALS_DIRECTORY`）を足す。常駐のときは、Secret Serviceより先にこれを読む
- フォージのインターフェースに、コメントの削除を足す
- DECISIONS.mdに、常駐の形、端末をまたいだ重複の防止、sudoを要する操作のADRを足す。READMEに、常駐の手順と、サブスクリプションの認証で常駐させるかは利用者の判断であることを書く
- 実物の2台で同時に動かす確認は、issueにして後回しにする

## Capabilities

### New Capabilities

- `service`: systemdのサービスとしての常駐（設定、クレデンシャル、確認、状態の表示）

### Modified Capabilities

- `agent-run`: 端末をまたいだ重複の防止（着手のコメントと、退いた端末の後始末）
- `config`: トークンの取り出しに、systemdのクレデンシャルを足す

## Impact

- `internal/agent` に着手のコメントと重複の判定を足し、`internal/forge`（GitHub、GitLab、偽物）にコメントの削除を足す
- `internal/secret` にsystemdのクレデンシャルの読み出しを足す
- 新しく `internal/service` を作り、unitの生成とsudoで実行するコマンドの組み立てを置く。`cmd/switchboard` に `service` のサブコマンドを足す
- 実行には、systemd（255で確認）、`systemd-creds`、sudoが要る
- README.md、DECISIONS.md、ROADMAP.mdを更新する
