# Tasks

## 1. 設定

- [ ] 1.1 `internal/config`：`<inbox.auth>` を足す。項目は `<client_id>`、`<client_secret_ref>`、`<member_groups>`、`<admin_groups>`、`<owner>` である。GitLabが無いとき、アプリケーションが空のとき、ログインできる人が空のときを誤りにする。`go test ./internal/config/` で確かめる
- [ ] 1.2 `config/switchboard.example.yaml`：字下げしたコメントで `<inbox.auth>` の例を足す。ADR-059を `docs/DECISIONS.md` に足す

## 2. GitLabのOIDC

- [ ] 2.1 `internal/webauth`：認可のURLの組み立て（`state`、PKCE）、`code` とトークンの交換、更新、ユーザー情報の読み取り、失効を作る。`httptest` の偽のGitLabで、要求の形と応答の読み取りを確かめる
- [ ] 2.2 `internal/webauth`：ログインできる人の判定を作る（グループのフルパスと下位のグループ、owner）。テーブルのテストで確かめる
- [ ] 2.3 `internal/webauth`：セッションの表を作る（作成、引き当て、24時間と7日の期限、更新とグループの確かめ直し、消去、掃除）。時計を差し替えたテストで確かめる

## 3. 受信箱のサービス

- [ ] 3.1 `inboxapi`：文脈にセッションIDを入れる関数と取り出す関数、`/v1/auth/` の要求と応答の型、符号 `unauthenticated` を足す。クライアントがヘッダ `X-Switchboard-Session` を付けることを、`go test ./inboxapi/` で確かめる
- [ ] 3.2 `internal/inboxserve`：`/v1/auth/status`、`start`、`callback`、`logout` を足す。認証を設定したとき、ほかの経路の前でセッションを確かめて文脈に入れ、無ければ401を返す。`go test ./internal/inboxserve/` で、認証なしの構成が変わらないことと併せて確かめる
- [ ] 3.3 `cmd/switchboard`：`inbox serve` が、`<inbox.auth>` から `internal/webauth` を作る。client secretは、Secret Serviceかクレデンシャルから読む。`go test ./cmd/switchboard/` で確かめる

## 4. 設置

- [ ] 4.1 `internal/service`：`service install --inbox` で、`<inbox.auth>` のclient secretをクレデンシャルにする。`go test ./internal/service/ ./cmd/switchboard/` で、表示に行があり、値は無いことを確かめる

## 5. Web表示

- [ ] 5.1 `web/internal/server`：`/auth/login`、`/auth/callback`、`/api/logout`、`/api/auth` を足す。受信箱のサービスを呼ぶときは、cookieのセッションIDを文脈に入れる。`web/` の `go test ./...` で確かめる。確かめるのは、cookieの属性、戻りの成功と失敗、ログアウトの `Origin` の検査である
- [ ] 5.2 `web/ui`：起動のときに `/api/auth` を読み、ログインしていなければログインの画面を出す。401の `unauthenticated` でログインの画面へ切り替え、SSEの再接続を止める。断られたときの理由とユーザー名の表示、ヘッダの名前とログアウトを足す。`pnpm test` で確かめる

## 6. Androidのアプリ

- [ ] 6.1 `app/`：WebViewで、`/api/auth` が返すGitLabの起点のページを開くようにする。`pnpm test` で、Web表示の起点とGitLabの起点は中で開き、ほかは外で開くことを確かめる

## 7. 文書と結合の確認

- [ ] 7.1 `docs/GUIDE.md`：GitLabのアプリケーションの登録、client secretの置き方、`<inbox.auth>` の書き方を足す。`docs/SECURITY.md` にHTTPでのcookieの危険を書く。textlintで確かめる
- [ ] 7.2 `docs/ROADMAP.md`：検証項目を足す。実物のGitLabでのログインと `groups` の形、アプリの外の `fetch` にcookieが付くか、の2つである
- [ ] 7.3 `make verify`：通ることを確かめる
