# Design

## Context

動機は proposal.md の Why を参照。今のコードでも、`relay.webhook_listen` と `relay.webhook_url` を書き換えればLANのアドレスで待てる。ただし、待ち受けのアドレスは何も検査しておらず、`0.0.0.0` も通る。`gitlab_token: true` は受け口の場所にかかわらず通る。WUIの待ち受けには、すでに `web/internal/webconfig` の検査（ADR-036）がある。

## Goals / Non-Goals

**Goals:**
- 受け口をLANのIPアドレスで待つことを、検査つきで認める
- LANでは、シークレットが回線に出ない署名トークンだけを使わせる
- 今の設定（既定の `127.0.0.1:8781`）をそのまま動かす

**Non-Goals:**
- 複数のGitLabインスタンス（ADR-030）
- 送り元のアドレスの制限、受け口のHTTPS、インターネットへの公開
- `sse_listen` の検査、WUIと受信箱のAPIの変更

## Decisions

- **検査は `validateRelay` に置き、既定値を入れたあとに走らせる。** 既定値の `127.0.0.1:8781` も同じ検査を通るので、書いていない設定も検査の対象になる。代わりに `relay serve` の起動時だけ検査する案もあったが、`doctor` や他のサービスの読み込みでも同じ誤りを出せるほうがよい
- **WUIの `webconfig.check` と同じ規則を、`internal/config` に別に書く。** `web/` は別のGoモジュールで、コアの `internal/config` を読まない。共有のパッケージを作るほどの量ではない。メッセージの言い回しも、キー名（`relay.webhook_listen`）を含める点が違う
- **ループバックの判定は `Relay.WebhookOnLoopback` にまとめ、設定の検査と `relay hook add` の両方で使う。** `::1` もループバックとして扱う。`webhook_listen` が空のとき（テストなどで既定値を入れていないとき）は既定の `127.0.0.1` とみなす
- **`gitlab_token` の拒否の理由は `config.GitLabTokenOffLoopback` の1か所で作る。** 設定の誤りと `relay hook add` の誤りで、同じ説明（平文のシークレットがLANを流れる、署名トークンを使う）を出す
- **`relay hook add --gitlab-token` は、シークレットを作る前に断る。** Secret Serviceとフォージに何も残さない

## Risks / Trade-offs

- [`0.0.0.0` やホスト名を書いていた既存の設定が読み込めなくなる] → メッセージで、この端末のLANのアドレスかIPアドレスを書くよう示す。今の文書は `127.0.0.1` だけを例にしており、該当する設定はまれと見る
- [LANの中でペイロードが平文で流れる] → 受け入れる（ADR-060）。中継が配るのはIDだけで、エージェントはフォージから取り直す（ADR-004）
- [別の機械のGitLabからの実機での確認ができない] → GUIDEの手順に沿って、利用者に確かめてもらう

## Migration Plan

設定を変えない利用者には作業はない。別の機械のGitLabから受けるときは、GUIDEの「別の機械のGitLabから受ける」に沿って `webhook_listen` を書き、`switchboard service install --relay` で置き直す。戻すときは、`webhook_listen` を消して置き直す。
