# Design

## Context

動機と範囲はproposal.mdに、判断はgrilling.mdにある。設計の前提になる今の形は、次のとおりである。

- 担当はプロジェクトの `host` で決まり、エージェントは自分の端末名のプロジェクトだけをポーリングする。issueのロックは、1台の中の重複しか防がない
- 実行の結果のコメントは、マーカーの `run` で二重投稿を防いでいる。同じ `run` のマーカーのコメントがあれば、投稿し直さない
- トークンは、Secret Serviceと、開発時の環境変数から読む。ログインしていないとgnome-keyringは解錠されない
- usknasのsystemdは255である。`systemd-creds --user` は256からなので、ユーザーサービスでは暗号化したクレデンシャルを読めない

## Goals / Non-Goals

**Goals:**

- 2台が同じissueに同時に着手しても、`claude -p` は1台でしか動かない
- 端末の再起動やSSHの切断の後も、エージェントが動き続ける。トークンは平文で置かない
- sudoで何をするかを、実行の前にすべて読める

**Non-Goals:**

- macOS（launchdとキーチェーン）、専用OSユーザーでの実行、エージェントの投稿アカウントの分離：別の変更で扱う
- 1つのプロジェクトを複数台で分け合うこと（grilling第1ラウンドQ2）
- 実物の2台での同時の確認：issueにして後回しにする

## Decisions

### 1. 着手のコメントで重複を見分ける

`agent:running` に付け替えたあと、エージェントは着手のコメントを投稿する。本文は短い定型文（「端末usknasで実行を始めます」）で、マーカーは `state=running`、`host`、`run` と、その実行がプロンプトに取り込む最後のコメントの `ack` を持つ。

投稿したら、issueを取り直す。同じ `ack` を持つ着手のコメントのうち、コメントIDが最も小さいものを探す。それが自分のものなら実行し、他の端末のものなら退く。2台は同じスレッドを読んで着手するので、`ack` が同じなら同じ契機での着手である。コメントIDはフォージが振る増えていく番号である。そのため、端末の時計を使わずに先後を決められる。

ラベルの付与時刻で範囲を決める案もある。しかし、GitHubとGitLabでラベルの付与時刻の取り方が違い、端末とフォージの時計のずれも入る。`ack` で揃えれば、どちらの問題も避けられる。

退いた端末は、自分の着手のコメントを消す（grilling第3ラウンドQ12）。ラベルは変えない。実行している端末が、結果に合わせて付け替える。

### 2. 二重投稿の防止は `state` も見る

着手のコメントと結果のコメントは、同じ `run` を持つ。今の二重投稿の防止のままでは、着手のコメントがあるだけで結果を投稿しなくなる。そのため、「同じ `run` で、同じ `state` のマーカーのコメントがあれば投稿しない」に改める。受信箱と、返答による再開の判定（`hasReply`）は、最新の `waiting` か `done` のマーカーを見る。そのため、着手のコメントの影響は受けない。この点はテストで確かめる。

### 3. コメントの削除

`forge.Forge` に `DeleteComment(ctx, ref, id)` を足す。GitHubでは `DELETE /repos/{o}/{r}/issues/comments/{id}` を呼ぶ。GitLabでは `DELETE /projects/:id/issues/:iid/notes/:note_id` を呼ぶ。偽物は、コメントを消して数え直す。

### 4. クレデンシャルからの読み出し

`secret.Resolver` に、`CREDENTIALS_DIRECTORY` を見る読み出し口を足す。クレデンシャルの名前は、`token_ref` の英数字以外を `_` に置き換えたもの（`switchboard/github` → `switchboard_github`）とする。systemdのクレデンシャルの名前に `/` を使えないためである。環境変数が無いか、ファイルが無ければ、今までどおりSecret Serviceを読む。

### 5. サービスの形

unitは `/etc/systemd/system/switchboard-agent.service` に置く。

```ini
[Unit]
Description=switchboard agent
After=network-online.target
Wants=network-online.target

[Service]
User=kanda
Environment=HOME=/home/kanda
Environment=PATH=<installを実行したときのPATH>
ExecStart=<switchboardの絶対パス> agent run --config <設定ファイルの絶対パス>
LoadCredentialEncrypted=switchboard_github:/etc/credstore.encrypted/switchboard.switchboard_github
Restart=on-failure
RestartSec=30s

[Install]
WantedBy=multi-user.target
```

`PATH` は、`claude` と `git` を見つけるために、`install` を実行したときの値を書き込む。`HOME` を書くのは、Claude Codeのサブスクリプションのログイン（`~/.claude`）と、状態ディレクトリを今と同じ場所にするためである。同じ状態ディレクトリを使うので、手で起動したエージェントとサービスが同時に動くことは、エージェントのロックが防ぐ。

### 6. `service install` が行うこと

1. 設定を読み、この端末の担当プロジェクトのエージェント用の `token_ref` を集める
2. それぞれのトークンをSecret Serviceから読む（このときkeyringが解錠されている必要がある）
3. sudoで実行するコマンドを、全部表示する
4. 確認を取る。`--dry-run` ならここで終わる。標準入力が端末でなく `--yes` も無ければ、ここで止める
5. 表示したとおりに、コマンドを1つずつsudoで実行する

sudoで実行するのは、次のコマンドである。

| コマンド | 目的 |
|---|---|
| `sudo install -d -m 0700 /etc/credstore.encrypted` | クレデンシャルの置き場所を作る |
| `sudo systemd-creds encrypt --name=<名前> - /etc/credstore.encrypted/switchboard.<名前>` | トークンを暗号化して置く。トークンは標準入力から渡し、引数には出さない |
| `sudo install -m 0644 <一時ファイル> /etc/systemd/system/switchboard-agent.service` | unitを置く |
| `sudo systemctl daemon-reload` | unitを読み込ませる |
| `sudo systemctl enable switchboard-agent.service` | 起動時に動くようにする |
| `sudo systemctl restart switchboard-agent.service` | 動かし直す（初めてなら起動する） |

`uninstall` は、`systemctl disable --now`、unitとクレデンシャルの `rm`、`daemon-reload` をsudoで実行する。`status` は、sudoを使わずに、unitのファイルと、`systemctl is-enabled` と `is-active` の結果を表示する。クレデンシャルの置き場所はrootしか読めないので、`status` ではunitに書いた名前を表示するだけにする。

コマンドの組み立ては `internal/service` に置き、実行は差し替えられる関数にする。テストでは、実際のsudoを呼ばずに、組み立てたコマンドと確認の流れを確かめる。

### 7. ADR

DECISIONS.mdに、次の3つを足す。

- ADR-032（常駐の形）：Linuxでは、`User=` を指定したsystemdのシステムサービスにする。エージェント用のトークンだけを、ホストの鍵で暗号化したクレデンシャルにして渡す
- ADR-033（端末をまたいだ重複の防止）：着手のコメントで、同じ `ack` の中で最も早く着手した端末だけが実行する。退いた端末は自分のコメントを消す
- ADR-034（sudoを要する操作）：製品のコマンドの中で行うが、実行するコマンドを事前に全部表示し、確認を取ってから1つずつ実行する

## Risks / Trade-offs

- ［着手のコメントで、issueのコメントが実行ごとに1件増える］→どの端末が動いたかがissueで分かる利点と引き換えにする。退いた端末のコメントは消す
- ［投稿と取り直しの間に、もう1台が投稿する］→取り直したあとで比べるので、遅れて投稿した側は、先に投稿した側のコメントを必ず見る。先に投稿した側が後の側を見落としても、IDの小さい自分が勝つので結果は変わらない
- ［サービスの `PATH` は、`install` のときの値に固定される］→`claude` を入れ直して場所が変わったら、`service install` をやり直す。READMEに書く
- ［トークンを入れ替えたら、クレデンシャルが古いままになる］→`service install` をやり直せば置き直す。READMEに書く
- ［サブスクリプションの認証で常駐させてよいかを確かめていない（V6）］→利用者の判断とし、READMEに書く（grilling第3ラウンドQ11）

## Migration Plan

- 常駐させない使い方（手で `agent run` を起動する）は、今までどおり動く
- 着手のコメントは、この版から付く。古い版のエージェントが同じプロジェクトを担当していると重複を見分けられないので、2台とも新しい版にする
- 戻すときは `service uninstall` を実行し、古い版のバイナリに入れ替える
