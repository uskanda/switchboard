# Proposal

## Why

中継のwebhookの受け口は、同じ端末のGitLabからだけ受ける前提で決まっている（ADR-007）。GitLabがLANの別の機械にあると、返答にすぐ着手する仕組みを使えない。受け口をLANのアドレスで待つことを正式に認め、そのときに必要な検査を足す。

## What Changes

- `relay.webhook_listen` に、この端末のLANのIPアドレスを書けるようにする（正式に認める）
- `relay.webhook_listen` を検査する。IPアドレスとポートの形でないもの、ホスト名、すべてのインターフェース（`0.0.0.0`、`::`）は、設定の誤りとして拒む。既定の `127.0.0.1:8781` は今までどおり通る
- `relay.webhook_listen` がループバックでないときは、`gitlab_token: true` の項目を設定の誤りとして拒む。`relay hook add --gitlab-token` も、シークレットを作る前に断る
- `relay hook add` の許可リストの案内を、LANの別の機械のGitLabにも当てはまる文にする
- ADR-060を足し、ADR-007とADR-017をこの点に限って改める。DESIGN、GUIDE、SECURITYを書き直す。GUIDEには、別の機械のGitLabから受ける手順と、私設のCAの証明書をOSの信頼ストアに入れる手順を書く

## Capabilities

### New Capabilities

なし

### Modified Capabilities

- `relay`：webhookの受け口がLANのIPアドレスを認め、すべてのインターフェースを拒む。ループバックでない受け口では、`X-Gitlab-Token` による検証と `relay hook add --gitlab-token` を認めない
- `config`：中継の設定の検証に、`webhook_listen` の検査と、ループバックでない受け口での `gitlab_token` の拒否を足す

## Impact

- コード：`internal/config/relay.go`、`internal/relay/admin.go`、`internal/inboxserve/projects.go`（コメントだけ）
- 文書：`docs/DECISIONS.md`、`docs/DESIGN.md`、`docs/GUIDE.md`、`docs/SECURITY.md`
- 互換：今の設定はそのまま動く。`webhook_listen` に `0.0.0.0` やホスト名を書いていた設定、LANのアドレスで `gitlab_token: true` を使っていた設定は、読み込みで拒まれる
- WUI、受信箱のAPI、SSEの配信口は変えない
