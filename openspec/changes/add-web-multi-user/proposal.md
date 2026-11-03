# Proposal

## Why

受信箱のWeb表示からの返答、issueの作成、マージは、すべて共有の受信箱のトークン1つで行われ、フォージには持ち主1人の操作として残る（issue #146）。そのため、複数の人で1つのswitchboardを共有すると、誰が指示したかを区別できない。トークンの持ち主でない人も、持ち主の名前でエージェントに信頼された指示を出せる。

## What Changes

- 認証の節 `<inbox.auth>`（Web表示の認証の変更で足す）のグループとownerから、memberとadminを決める
- 認証を設定したとき、Web表示とアプリからのGitLabのプロジェクトへの書き込み（返答、計画の承認、issueの作成、マージ）を、ログインした人のGitLabのOAuthのトークンで行う
- 認証を設定したとき、GitHubのプロジェクトは、ownerにだけ見せ、ownerだけが共有の受信箱のトークンで書き込める
- 一覧、1件の表示、SSEから、ログインした人が読めないGitLabのプロジェクトを外す
- 追加の指示の編集とプロジェクトの追加を、adminに限る
- `trusted_authors` に無い人が返答や承認をしようとするとき、Web表示が、エージェントに指示として扱われないことを先に知らせる
- 認証を設定したとき、エージェントの `agent_identity` が `trusted_authors` にあれば、設定の誤りにする（**BREAKING**：認証を有効にする構成だけが対象）
- `switchboard doctor` が、共有の受信箱のトークン（GitLab）の書き込みのスコープと、ownerと `trusted_authors` の食い違いを警告する
- Web表示からの書き込みのたびに、操作したGitLabのユーザー名を監査ログに残す
- 認証を設定したとき、Androidの通知の「承認」と「マージ」は、アプリを開いて確認の画面を出す
- `<inbox.auth>` が無い構成は、今の振る舞いのまま変えない

ログイン、セッション、OAuthのトークンの取得は、Web表示の認証（issue #15）の変更で作る。この変更は、受信箱のサービスが要求ごとに、ログインした人のユーザー名、グループ、GitLabのアクセストークンを引けることを前提にする。

## Capabilities

### New Capabilities

- `web-users`: 認証を設定した受信箱での、利用者の権限、書き込みの身元、見せるプロジェクトの範囲、信頼されない人への知らせ、監査の記録

### Modified Capabilities

- `config`: 認証を設定したときのエージェントの身元の検証
- `trust`: エージェントと信頼するユーザーが同じアカウントのときの特例（ADR-056）を、認証を設定しない構成に限る

## Impact

- `internal/config`：エージェントの身元の検証を足す
- `internal/inboxserve`：書き込みの経路で使うトークンを、要求の利用者で選ぶ。一覧、1件、SSEを利用者で絞る。adminだけの経路を断る
- `internal/doctor`：警告を足す
- `internal/agent`：ADR-056の特例を、認証を設定しない構成に限る
- `web/ui`：GitHubのプロジェクトの書き込みのボタンを、owner以外に出さない。信頼されない人への知らせを出す
- `app/`：認証を設定したときの通知のボタンの動き
- docs：ADR-060を足す（ADR-015、ADR-040、ADR-043、ADR-056を改める）。利用者の記述（`docs/GUIDE.md`、`docs/SECURITY.md`、`PRODUCT.md`）
- 依存：Web表示の認証（issue #15）の変更（ログインと利用者の文脈）
