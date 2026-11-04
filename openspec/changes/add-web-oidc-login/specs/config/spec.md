# Spec Delta

## ADDED Requirements

### Requirement: 受信箱の認証の設定
`config.yaml` の `<inbox.auth>` に、GitLabのOpenID Connectのアプリケーションと、ログインできる人を書けなければならない（MUST）。
アプリケーションは、client id（`<client_id>`）と、client secretのSecret Serviceでの名前（`<client_secret_ref>`）で書く。
ログインできる人は、GitLabのグループのフルパスの列（`<member_groups>`、`<admin_groups>`）と、GitLabのユーザー名1つ（`<owner>`）で書く。
`<inbox.auth>` を書いたとき、次のどれかなら設定の誤りとしなければならない（MUST）。

- `forges.gitlab` が無い
- `<client_id>` か `<client_secret_ref>` が空である
- `<member_groups>`、`<admin_groups>`、`<owner>` がどれも空である

#### Scenario: GitLabの無い構成で認証を書く
- **WHEN** `forges.gitlab` の無い設定に `<inbox.auth>` を書く
- **THEN** コマンドは0以外の終了コードで終わり、認証にはGitLabの設定が要ると報告する

#### Scenario: ログインできる人を書いていない
- **WHEN** `<inbox.auth>` に `<client_id>` と `<client_secret_ref>` だけを書く
- **THEN** コマンドは0以外の終了コードで終わり、ログインできる人を書くよう報告する
