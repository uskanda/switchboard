# Tasks

## 1. 設定の検査

- [x] 1.1 `internal/config/relay_test.go` に、LANのアドレスが通り `webhook_url` の既定が `http://192.168.0.82:8781` になるテスト、`0.0.0.0:8781`、`[::]:8781`、`localhost:8781`、ポートの無いものを拒むテスト、LANのアドレスで `gitlab_token` を拒み `127.0.0.1` と `[::1]` では通すテストを足す
- [x] 1.2 `internal/config/relay.go` の `validateRelay` に `webhook_listen` の検査とループバックでない受け口での `gitlab_token` の拒否を足し、1.1のテストと既存のテストが `make verify` で通ることを確かめる

## 2. relay hook add と案内

- [x] 2.1 `internal/relay/admin_test.go` に、LANのアドレスの設定で `--gitlab-token` が断られ、Secret Serviceとフォージに何も書かれないテストを足す
- [x] 2.2 `internal/relay/admin.go` で `--gitlab-token` をシークレットを作る前に断り、許可リストの案内を別の機械のGitLabにも当てはまる文に直し、2.1と既存のテストが `make verify` で通ることを確かめる
- [x] 2.3 `internal/inboxserve/projects.go` のADR-007を前提にしたコメントを直す

## 3. 文書

- [x] 3.1 `docs/DECISIONS.md` にADR-059を足し、ADR-007とADR-017に改めた旨を書き添える
- [x] 3.2 `docs/DESIGN.md` のwebhook受信の項とADR-007の段落を書き直す
- [x] 3.3 `docs/GUIDE.md` に、別の機械のGitLabから受ける手順と、私設のCAの証明書をOSの信頼ストアに入れる手順を書く
- [x] 3.4 `docs/SECURITY.md` のT5に、中継の受け口をLANで待つ場合の扱いを足す

## 4. 確認

- [x] 4.1 `openspec validate relay-lan-webhook --strict` と `make verify` が通ることを確かめる
