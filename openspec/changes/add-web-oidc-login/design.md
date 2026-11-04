# Design

## Context

Web表示（`switchboard-web`）は、LANのアドレスのHTTPで待つ。すべての要求は、Unixソケットの `<inboxapi>` を通して受信箱のサービス（`switchboard inbox serve`）へ渡す。Web表示はフォージのトークンを持たない（ADR-037）。ソケットを使うのはWeb表示だけで、CLIはフォージを直接呼ぶ。

ログインで得るGitLabのアクセストークンは、`api` のスコープを持つフォージのトークンである。そのため、トークンの取得と保持は受信箱のサービスで行う。

## Goals / Non-Goals

**Goals:**

- 認証を設定したとき、ログインしていない人にWeb表示の中身を見せない
- 受信箱のサービスが、要求ごとにログインした人の文脈（ユーザー名、グループ、アクセストークン）を引けるようにする。`add-web-multi-user` は、これを使う

**Non-Goals:**

- 書き込みの身元と、memberとadminの権限の中身（`add-web-multi-user`）
- HTTPS（#155）

## Decisions

### OIDCは受信箱のサービスで行う

ソケットの経路を `/v1/auth/` に足す。

| 経路 | 役割 |
|---|---|
| `GET /v1/auth/status` | 認証を設定しているか。セッションがあれば、そのユーザー名 |
| `POST /v1/auth/start` | `state` とPKCEの `code_verifier` を作って10分間持ち、認可のURLを返す |
| `POST /v1/auth/callback` | `state` を確かめ、`code` をトークンと交換し、ユーザー情報を読み、ログインできる人ならセッションIDを返す |
| `POST /v1/auth/logout` | セッションを消し、`/oauth/revoke` でトークンを失効させる |

戻り先のURLは、Web表示が `web.yaml` の `listen` から作って、`start` と `callback` に渡す。受信箱のサービスは `listen` を知らないためである。GitLabのアプリケーションには、同じURLを登録してもらう。

GitLabの発見の文書（`/.well-known/openid-configuration`）は読まない。代わりに、`base_url` の下の決まったパスを使う。パスは `/oauth/authorize`、`/oauth/token`、`/oauth/userinfo`、`/oauth/revoke` である。GitLabのパスは変わらず、依存を1つ減らせる。外部の仕様に触れる箇所は、新しい `internal/webauth` に閉じ込める。

ユーザー情報は `/oauth/userinfo` から読む。ユーザー名は `nickname`、グループは `groups`（フルパスの列）である。IDトークンの署名の検証はしない。トークンは、client secretを使ってGitLabのトークンのエンドポイントから直接受け取る。ユーザー情報も、そのトークンでGitLabから読む。そのため、署名を検証しても守れるものが増えない。

### セッションIDの受け渡し

`<inboxapi>` に、文脈へセッションIDを入れる関数と取り出す関数を足す。クライアントは、文脈にセッションIDがあれば、ヘッダ `X-Switchboard-Session` に付けて送る。Web表示は、cookieの値を文脈に入れてから、今までどおりクライアントのメソッドを呼ぶ。メソッドの引数は変えない。

受信箱のサービスは、認証を設定しているとき、`/v1/auth/` 以外の経路の前でヘッダのセッションを確かめる。無いか消えていれば、401と符号 `unauthenticated` を返す。確かめたセッションは、要求の文脈に入れる。`add-web-multi-user` は、そこから利用者の文脈を取り出す。

### セッションの表

セッションの表は、受信箱のサービスのメモリーの `map` で持ち、セッションIDは32バイトの乱数の16進数とする。1つのセッションは、ユーザー名、グループ、アクセストークン、更新用のトークン、トークンの期限、作った時刻、最後の要求の時刻を持つ。

トークンの更新は、要求のときに行う。アクセストークンの期限まで5分を切っていたら、更新してから要求を進める。更新のたびにユーザー情報を読み直し、ログインできる人でなくなっていれば、セッションを消して401を返す。期限の切れたセッションは、1分ごとの掃除と、要求のときの確認で消す。

### Web表示

Web表示に、次の経路を足す。

- `GET /auth/login`：受信箱のサービスの `start` を呼び、認可のURLへ302で移す
- `GET /auth/callback`：`callback` を呼ぶ。成功ならcookie `sb_session` を付けて `/` へ移す。断られたら、理由とユーザー名を付けて画面の `/login` へ移す
- `POST /api/logout`：`Origin` と `Host` を確かめてから `logout` を呼び、cookieを消す
- `GET /api/auth`：`status` をそのまま返す。画面は、起動のときにこれを読み、ログインしていなければログインの画面を出す

cookieは `HttpOnly`、`SameSite=Lax`、`Path=/` とし、`Secure` は付けない（HTTPのため）。期限は付けず、セッションの期限は受信箱のサービスが決める。GitLabからの戻りはトップレベルのGETなので、`SameSite=Lax` のcookieが届く。

画面（`web/ui`）は、401の `unauthenticated` を受けたら、ログインの画面に切り替える。SSEは401のときに再接続を止める。

### Androidのアプリ

WebViewの `onShouldStartLoadWithRequest` で、Web表示の起点に加えて、GitLabの起点のページもWebViewの中で開く。GitLabの起点は、Web表示の `/api/auth` が返す値から知る。アプリは、GitLabの起点を設定として持たない。WebViewのcookieは、Androidの既定で端末に残る。

アプリの外の `fetch`（端末の登録と、通知からの承認）にセッションのcookieが付くかは、確かめていない。ReactNativeのAndroidの通信は、WebViewと同じcookieの置き場所を使うはずである。確かめた結果は検証項目に残す。付かなければ、端末の登録をWebViewの画面から行う形に変える。

### 設定と秘密

`<inbox.auth>` に、`client_id`、`client_secret_ref`、`member_groups`、`admin_groups`、`owner` を置く。client secretは、ほかのトークンと同じくSecret Serviceから読み、`service install --inbox` でクレデンシャルにする。

### ADR

ADR-059を足し、ADR-036の「当面は利用者を認証しない」を、認証を設定した構成について改める。

## Risks / Trade-offs

- リスク：HTTPなので、同じLANでcookieを盗み見た人が、その人として操作できる。対策：受け入れ、`docs/SECURITY.md` に書く。HTTPSは、issue #155で扱う。
- リスク：受信箱のサービスを再起動すると、全員がログインし直しになる。対策：受け入れる。GitLabにログイン済みなら、確認の画面を1回通るだけで済む。
- リスク：GitLabのユーザー情報の `groups` の形が版によって違う。対策：`internal/webauth` の中で読み、実物のGitLabで確かめた結果を検証項目に残す。
- リスク：アプリの外の `fetch` にcookieが付かず、端末の登録が401になる。対策：検証項目で確かめ、付かなければ登録の経路を変える。

## Migration Plan

1. GitLabの管理画面か利用者の設定で、アプリケーションを登録する。confidentialで、スコープは `openid`、`profile`、`api`、戻り先は `http://<listen>/auth/callback` である
2. client secretをSecret Serviceに置く
3. `<inbox.auth>` を書き、`switchboard service install --inbox` で置き直す
4. 戻すときは、`<inbox.auth>` を消して受信箱のサービスを置き直す
