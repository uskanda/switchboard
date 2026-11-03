# Tasks

この変更は、Web表示の認証（issue #15、`add-web-oidc-login`）の上に作る。

## 1. 設定と検証

- [x] 1.2 `internal/config`：認証を設定したとき、エージェントの身元が同じフォージの `trusted_authors` にあれば、誤りにする。身元は、フォージとプロジェクトごとの `agent_identity` である。テストで、GitLabとGitHubの両方の重なりと、認証なしでは誤りにならないことを確かめる
- [x] 1.3 `docs/GUIDE.md` に複数の利用者の設定の例を足し、ADR-060を `docs/DECISIONS.md` に足す。`make verify` の文書の検査が通ることで確かめる

## 2. 権限の判定

- [x] 2.1 `internal/inboxserve`：利用者の文脈から、その人がadmin、member、利用者でない人のどれかを決める関数を足す。ownerは、所属のグループを問わずadminとする。テーブルのテストで確かめる
- [x] 2.2 `internal/inboxserve`：利用者でない人の要求を、すべての経路で断る。追加の指示とプロジェクトの経路を、admin以外に断る。`go test ./internal/inboxserve/` で、経路ごとに確かめる

## 3. 書き込みの身元

- [x] 3.1 `internal/inboxserve`：書き込みの経路（返答、承認、issueの作成、マージ、draftの解除）で、利用者の文脈から書き手のクライアントを選ぶ。GitLabは利用者のトークン、GitHubはownerだけ共有のトークン、それ以外は権限の誤りとする。偽のフォージで、投稿に使われたトークンを確かめるテストを置く
- [x] 3.2 `internal/inboxserve`：フォージが権限の不足で断ったときに、共有のトークンで再試行せず、権限の誤りを返すことをテストで確かめる
- [x] 3.3 `internal/inboxserve`：認証なしの構成では、すべての書き込みが共有のトークンのままであることを、既存のテストが変わらず通ることで確かめる

## 4. 見せる範囲

- [x] 4.1 `internal/inboxserve`：利用者のトークンで、`GET /projects/:id` を呼んで読めるかを判定し、利用者とプロジェクトの組で10分間キャッシュする。誤りは読めないとしてキャッシュしない。時計を差し替えたテストで確かめる
- [x] 4.2 `internal/inboxserve`：一覧、1件の表示、issueを作れる先、マージの状態、アイコンの各応答を、読めるプロジェクトとGitHubのowner限定で絞る。経路ごとに、読めないプロジェクトが出ないテストを置く

## 5. 信頼の知らせとWeb表示

- [x] 5.1 1件の表示の応答に、ログインした人の返答が指示として扱われるかを足し、`<inboxapi>` の型に反映する。`go test ./internal/inboxserve/ ./inboxapi/` で確かめる
- [x] 5.2 `web/ui` で、扱われないときに返答の欄の上へ知らせを出す。owner以外にはGitHubのプロジェクトの書き込みのボタンを出さず、admin以外には設定の画面への導線を出さない。`pnpm test` のコンポーネントのテストで確かめる
- [x] 5.3 `internal/inboxserve`：一覧の応答に認証の有無を足す。`web/` の `go test` で、Web表示がそれを画面に渡すことを確かめる

## 6. 自動のマージ

- [x] 6.1 `internal/automerge`：自動のマージの記録に頼んだ人を足し、更新の手段をメモリーの表に持つ。表に手段が無い記録は、再起動の理由で取り消して結果の知らせに載せる。`go test ./internal/inboxserve/ ./internal/automerge/` で、再起動の後の取り消しを確かめる
- [x] 6.2 `internal/inboxserve`：自動のマージを、頼んだ人の身元で行うことを、偽のフォージのテストで確かめる

## 7. 監査ログ

- [x] 7.1 `internal/inboxserve`：Web表示からの書き込みのたびに、監査ログに `web.action` の行を足す。行には `actor` を書く。対象は、返答、承認、issueの作成、マージ、自動のマージの取り消し、追加の指示の保存、プロジェクトの追加、端末の登録である。認証なしでは足さないことも、テストで確かめる

## 8. エージェントとdoctor

- [x] 8.1 ADR-056の特例を、`<inbox.auth>` の無い構成に限る。`go test ./internal/agent/` で、認証ありの構成でエージェントのアカウントのラベルから実行が始まらないことを確かめる
- [x] 8.2 `internal/doctor`：`switchboard doctor` に警告を3つ足す。1つ目は、GitLabの共有の受信箱のトークンが `api` のスコープを持つときである。2つ目はownerが `trusted_authors.gitlab` に無いとき、3つ目はGitHubの受信箱のトークンの持ち主が `trusted_authors.github` に無いときである。`go test ./internal/doctor/` で確かめる

## 9. Androidのアプリ

- [x] 9.1 `app/`：通知のボタンを押されたとき、最後に読んだ一覧が認証ありを示すか、まだ読んでいなければ、アプリを開いてそのissueを出す。`app/` の `pnpm test` で、認証あり、なし、未読の3通りを確かめる

## 10. 文書と結合の確認

- [x] 10.1 `docs/GUIDE.md` に、複数の利用者での設置の手順（botの用意、`read_api` のトークン、`<inbox.auth>` の書き方、移り方）を足す。`docs/SECURITY.md` と `PRODUCT.md` の利用者の記述を直す。textlintで確かめる
- [x] 10.2 `make verify` を通す
- [x] 10.3 `docs/ROADMAP.md` に検証項目を足す。項目は、実物のGitLabで、2人のmemberのそれぞれの返答がその人のコメントとして残り、読めないプロジェクトが一覧に出ないことである
