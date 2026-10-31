# Design

## Context

動機と範囲はproposal.mdに、判断はgrilling.mdにある。設計の前提になる今の形と事実は、次のとおりである。

- エージェントは `poll_interval` ごとに `Poll` を呼ぶ。`Poll` は、担当プロジェクトごとに `UpdatedIssues` を取り、GitLabでは待ちのissueのノートの数も見る（V22）。評価済みかどうかは、issueごとの更新時刻とノートの数で判定する
- トークンは `secret.Resolver` で読む。systemdのクレデンシャルを先に見て、無ければSecret Serviceを読む（ADR-032）
- `internal/service` は、エージェントのunitを作り、sudoで実行するコマンドを全部表示してから確認を取る（ADR-034）
- `gl.uskan.dev`（CE 19.5.1）では、`uskanda` は管理者である。ローカルネットワークへのwebhookは許されておらず、許可リストは空である。`nas/switchboard-sandbox` にwebhookは無い
- usknasのTailscaleは止まっていて、公開の受け口は無い。GitHub.comからのwebhookは、当面受けられない

## Goals / Non-Goals

**Goals:**

- 中継につながっている間、返答から実行の開始までの遅延が、ポーリングの間隔に依存しない
- 中継が止まっても、エージェントはポーリングで処理を続ける
- 中継は、IDより多くを知らない。侵害されても、流せるのは「取りに行け」という合図だけである（ADR-004）

**Non-Goals:**

- 公開の受け口と別のサーバー（ADR-007は、当面は公開しないと決める）
- GitHubの実物での確認（issueにする）
- 受信箱の中継対応。受信箱は今までどおり中継に依存しない

## Decisions

### 1. パッケージの分け方

- `internal/relay`：webhookの受け口、署名の検証、重複の除去、抽出、SSEの配信。フォージごとの処理は、`github.go` と `gitlab.go` の小さな関数に分ける。フォージのAPIクライアントには依存しない
- `internal/relayclient`：SSEの購読、`Last-Event-ID`、再接続の間隔
- `internal/forge`：webhookの登録と削除のために、`Hooks` インターフェース（`CreateHook`、`DeleteHook`）を `Forge` とは別に足す。GitHubとGitLabのクライアント、偽物が実装する。エージェントの経路は `Hooks` を使わない

### 2. 設定の形

```yaml
relay:
  webhook_listen: 127.0.0.1:8781
  sse_listen: 127.0.0.1:8782
  webhook_url: http://127.0.0.1:8781   # フォージに登録するURLの頭。公開の受け口を作ったら変える
  hooks:
    - project: gl:nas/switchboard-sandbox
      key: <hook-key>
      secret_ref: switchboard/relay-hook-<hook-key>
  hosts:
    - name: usknas
      token_hash: sha256:<16進>
agent:
  poll_interval: 30s
  reconcile_interval: 10m
  relay:
    url: http://127.0.0.1:8782/events
    token_ref: switchboard/relay-usknas
```

`hook-key` は、24バイトの乱数をbase64urlにした値である。秘密ではないが、推測できないので、知らないパスへの総当たりを404で切れる。署名の検証が本体の防御である。

設定ファイルを書き換えると、コメントや並びが崩れる。そのため、`relay hook add` と `relay token add` は設定を書き換えず、足す項目を表示する。利用者が貼り付ける。

### 3. 受信の処理

処理の順は、ボディの上限（5 MiB、`http.MaxBytesReader`）、`hook-key` の照合、署名の検証、配信IDでの重複の除去、対象のイベントの抽出である。

- GitHubは、`X-Hub-Signature-256` の `sha256=<16進>` を、ボディのHMAC-SHA256と `hmac.Equal` で比べる
- GitLabの署名トークンは、Standard Webhooksの形である。`webhook-id.webhook-timestamp.ボディ` のHMAC-SHA256をbase64にし、`webhook-signature` の `v1,<base64>` のどれかと比べる。タイムスタンプが±5分を外れたら破棄する。鍵は、`whsec_` を除いた残りをbase64で解いた32バイトである（V16で確かめた）
- `gitlab_token: true` のプロジェクトだけは、`X-Gitlab-Token` をシークレットと `subtle.ConstantTimeCompare` で比べる
- 重複の除去は、配信IDから受信時刻への表と、72時間を超えたものを消す処理で行う。件数の上限（10万件）を超えたら古いものから消す
- 抽出は、ペイロードを、必要な項目だけを持つ構造体へデコードする。GitHubで読むのは、`action`、`issue.number`、`repository.full_name` である。GitLabで読むのは、`object_kind` と `project.path_with_namespace` である。issueの番号は、Issue Hookでは `object_attributes.iid` から読む。Note Hookでは `issue.iid` から読み、`object_attributes.noteable_type` でissueへのものかを見る。本文などは構造体に入れないので、デコードの段階で捨てられる

応答は、成功と重複と対象外が200、知らない `hook-key` が404、署名の失敗が401、ボディの上限が413である。ログには、配信ID、フォージ、プロジェクト、issue番号、種別、結果だけを書く。

### 4. SSEの配信

`GET /events` に `Authorization: Bearer <token>` を付けてつなぐ。中継は、トークンのSHA-256を、設定の `hosts` のハッシュと定数時間で比べる。

イベントは、起動からの通し番号を `id:` に付ける。

```
id: 1042
event: issue
data: {"forge":"gl","project":"nas/switchboard-sandbox","issue":4,"kind":"note","delivery":"<uuid>"}
```

直近1,000件をリングバッファに持ち、`Last-Event-ID` より後のものを先に送る。中継を起動し直すと番号は1から始まり、バッファも空になる。エージェントは、つながるたびに照合を1回行う。そのため、取りこぼしたイベントのissueも、照合で評価される。接続を保つために、30秒ごとにコメント行（`: ping`）を送る。

### 5. エージェントの中継モード

`<agent.relay>` があれば、`Run` は購読のゴルーチンを起動する。

- つながっている間の照合の間隔は `reconcile_interval`、切れている間は `poll_interval` である。ループは、間隔が変わったらtickerを作り直す
- 届いたイベントは、担当プロジェクトでなければ捨てる。担当なら、プロジェクトごとの3秒のタイマーを張り直し、時間が来たらそのプロジェクトだけを `Poll` と同じ処理で取り直す。issue単位ではなくプロジェクト単位にするのは、今の評価の判定（更新時刻とノートの数）をそのまま使えるからである。費用は、イベントごとに1〜2回の問い合わせである
- つながったら、すぐに全プロジェクトを照合する
- 切れたら、ログに残してポーリングの間隔に戻る。再接続は1秒から始め、倍にしながら最大60秒の間隔で試みる

`Poll` と、イベントによる取り直しは、同じロックで直列にする。同じissueを2つの経路から同時に評価しないためである。

### 6. webhookの登録

`relay hook add <project>` は、次の順に行う。

1. `hook-key` とシークレットを作り、シークレットを `switchboard/relay-hook-<hook-key>` としてSecret Serviceに置く
2. 受信箱のトークンで、`<webhook_url>/webhook/<forge>/<hook-key>` を登録する
   - GitHub：`POST /repos/{o}/{r}/hooks`。`events` は `issues` と `issue_comment`、`content_type` は `json`、`secret` にシークレットを入れる
   - GitLab：`POST /projects/:id/hooks`。`issues_events` と `note_events` を有効にし、`signing_token` に署名トークン（`whsec_<base64>`）を設定する（V16で確かめた）
3. 403か404なら、画面での登録の手順を表示する。手順には、URL、対象のイベント、シークレットの値（ここで1回だけ）を含める。GitLabでは、許可リストに足す `<127.0.0.1:8781>` も示す
4. 設定の `<relay.hooks>` に足す項目を表示する

`relay hook remove <project>` は、登録したwebhookを、URLが一致するものを探して消し、シークレットをSecret Serviceから消す。

### 7. 中継のサービス

`service install --relay` は、`<switchboard-relay.service>` をエージェントと同じ形で置く。`User=`、`LoadCredentialEncrypted=`、`Restart=on-failure` を使う。クレデンシャルは、`<relay.hooks>` の `secret_ref` である。端末のトークンについては、設定にハッシュを書くだけなので、クレデンシャルは要らない。

エージェントのサービスでは、`<agent.relay.token_ref>` もクレデンシャルにする。

### 8. ADR

- ADR-007（中継の公開方法）：当面は公開しない。受け口は `127.0.0.1` で待ち、同じ端末のGitLabだけから受ける。公開するときは、別のサーバーを用意して改めて決める
- ADR-017（GitLabのwebhookの検証方式）：署名トークンを優先する。V16の結果に従い、使えないプロジェクトに限って `X-Gitlab-Token` を許す
- ADR-035（中継の認証とシークレット）：SSEは端末ごとのbearerトークンで認証し、中継はハッシュだけを持つ。webhookのシークレットはプロジェクトごとに作り、`hook-key` で引く。登録は受信箱のトークンで行い、エージェントのトークンには権限を足さない

## Risks / Trade-offs

- ［GitLabは、webhookの失敗が続くと、そのwebhookを一時的に止める］→中継を止めている間に失敗すると、webhookは止まりうる。止まっている間の更新も照合で評価されるので、処理は失われない。止まったwebhookは、GitLabの画面で有効にし直す。READMEに書く
- ［中継を起動し直すと、イベントの番号が戻る］→エージェントは、つながるたびに照合する
- ［署名トークンの鍵の形が、文書と違う］→V16で実物から確かめてから、検証を実装する
- ［同じ端末の他のユーザーが、受け口にwebhookを偽って送る］→署名がなければ破棄する。配信口はbearerトークンで守る
- ［GitLabは、許可リストを変えてから数分の間、Sidekiqから送るイベントを `BlockedUrlError` で止める（V16）］→許可リストを足したら、数分待ってから試す。READMEに書く
- ［プロジェクト単位の取り直しは、issue単位より問い合わせが多い］→3秒のまとめと、個人の規模のプロジェクト数なら問題にならない

## Migration Plan

- `<agent.relay>` が無い設定は、今までどおりポーリングだけで動く
- 中継を使い始めるときは、まず `relay token add` と `relay hook add` を実行し、表示された項目を設定に貼り付ける。次に、`service install --relay` で中継を置き、`service install` でエージェントを中継モードで置き直す
- 戻すときは、設定から `<agent.relay>` を消して `service install` をやり直す。そのあと、`service uninstall --relay` と `relay hook remove` を実行する
