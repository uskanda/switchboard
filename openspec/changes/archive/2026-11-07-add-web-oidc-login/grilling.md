# grilling 記録: add-web-oidc-login

issue #15。複数の利用者での共有（#146、変更 `add-web-multi-user`）のgrillingで、ログインをこの変更に分けると決めた。KeycloakのSSOを想定していたが、当面はGitLabのOpenID Connectを使う（Keycloakは #151）。

## 決定

| 決定 | 選択 | 出典 |
|---|---|---|
| ログインの方法 | GitLabのOpenID Connectを使う | `add-web-multi-user` 第1ラウンドへの返答 |
| この変更の範囲 | ログイン、セッション、ログインできる人の判定を扱う。書き込みの身元と権限の中身は `add-web-multi-user` で扱う | `add-web-multi-user` 第2ラウンド Q1 |
| ログインできる人 | `<member_groups>`、`<admin_groups>` のGitLabのグループに属する人と、`<owner>` の1人 | `add-web-multi-user` 第2ラウンド Q4、第4ラウンド Q15 |
| トークンの持ち方 | 受信箱のサービスのメモリーにだけ置く | `add-web-multi-user` 第3ラウンド Q10 |
| OIDCの処理の場所 | 受信箱のサービスが行う。Web表示は、認可コードを受信箱のサービスへ渡し、セッションIDだけをcookieにする | 第1ラウンド Q1 |
| アプリケーションの登録と秘密 | GitLabにconfidentialのアプリケーションを登録する。スコープは `openid`、`profile`、`api`。client secretはSecret Serviceとsystemdのクレデンシャルで渡し、PKCEも使う | 第1ラウンド Q2 |
| HTTPのままでよいか | 当面はHTTPのままにする。cookieは `HttpOnly` と `SameSite=Lax` にし、危険を `docs/SECURITY.md` に書く。HTTPSは #155 に分ける | 第1ラウンド Q3 |
| セッションの期限 | 最後の操作から24時間と、ログインから7日の早いほう。トークンの更新のたびにグループの所属を確かめ直し、外れていたら消す | 第1ラウンド Q4 |
| ログインを求める範囲 | `/auth/` と画面の静的ファイルを除く、すべての `/api/` とSSE。ログインしていなければ401を返し、画面はログインの画面へ移る | 第1ラウンド Q5 |
| Androidのアプリでのログイン | GitLabのページだけはWebViewの中で開き、cookieをWebViewに保存する | 第1ラウンド Q6 |
| ログアウト | ヘッダに名前とログアウトを出す。ログアウトでセッションを消し、`/oauth/revoke` でトークンを失効させる | 第1ラウンド Q7 |
| 認証を設定しないとき | 今の振る舞いのまま変えない | 第1ラウンド Q8 |
| ログインの画面 | 「GitLabでログイン」のボタンだけの画面を出す。断られた人には、権限が無いことと、ログインしたGitLabのユーザー名を出す | 第2ラウンド Q9 |
| ログインに使うGitLab | GitLabの `base_url` のGitLabに固定する | 第2ラウンド Q10 |

## 後回しにしたもの

- HTTPS（#155）
- KeycloakのSSO（#151）
- Custom Tabsでのアプリのログイン

## 状態

frontierは空。共有理解は2026-11-03に確認済み。
