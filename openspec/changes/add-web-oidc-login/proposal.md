# Proposal

## Why

受信箱のWeb表示は、利用者を認証しない（ADR-036、issue #15）。そのため、LANの中の機器なら誰でも一覧と質問の全文を読める。利用者の名前で返答でき、issueを作って実行も起こせる（ADR-040）。複数の人で共有するには、まず誰が使っているかを確かめる必要がある（issue #146）。

## What Changes

- `config.yaml` の `<inbox.auth>` に、GitLabのOpenID Connectのアプリケーションを書けるようにする。書くのは、client idとclient secretの参照である
- 同じ節に、ログインできる人（`<member_groups>`、`<admin_groups>`、`<owner>`）を書けるようにする
- 認証を設定したとき、受信箱のサービスがGitLabのOIDCでのログインを行い、セッションをメモリーに持つ。トークンは受信箱のサービスから出さない
- Web表示に、ログインの画面、GitLabからの戻り、ログアウトを足す。セッションIDだけを `HttpOnly` のcookieにする
- 認証を設定したとき、ログインしていない人の `/api/` の要求とSSEを401で断る
- ヘッダに、ログインした人の名前とログアウトを出す
- Androidのアプリが、GitLabのログインのページをWebViewの中で開く
- `switchboard service install --inbox` が、client secretをクレデンシャルとして受信箱のサービスに渡す
- `<inbox.auth>` が無い構成は、今の振る舞いのまま変えない

## Capabilities

### New Capabilities

- `web-auth`: 受信箱のWeb表示へのログイン、セッション、ログインできる人の判定、ログアウト

### Modified Capabilities

- `config`: `<inbox.auth>` の書き方と検証
- `service`: 受信箱のサービスに、client secretのクレデンシャルを渡す

## Impact

- `internal/config`：`Inbox` に認証の節を足す
- 新しい `internal/webauth`：GitLabのOIDCのクライアントと、セッションの表
- `internal/inboxserve`：`/v1/auth/` の経路と、セッションの確認
- `inboxapi`：セッションIDを要求に付ける手段と、ログインの型
- `web/internal/server`、`web/ui`：ログインの画面、戻り、ログアウト、401の扱い
- `app/`：GitLabのページをWebViewで開く
- `internal/service`、`cmd/switchboard`：クレデンシャルの受け渡し
- docs：ADR-059を足し、ADR-036を改める。`docs/GUIDE.md` に設置の手順、`docs/SECURITY.md` にHTTPの危険を書く
