# Design

## Context

動機はproposal.mdの「Why」を参照。今のコードで、この設計を縛る点は次のとおり。

- 受信箱のサービス（`internal/inboxserve`）は、受信箱のトークンのクライアントを `forge.Forge` として持つ。プロジェクトの追加（ADR-039）では、`forge.Repos` という別の小さなインターフェースで、リポジトリを読む
- 設定の読み直し（ADR-039）では、`Server.Reload` が新しい設定を受け、各部品の設定を差し替える
- WUI（`web/`）は核と別のモジュールで、`inboxapi` とUnixソケットだけで核とつながる（ADR-037）。POSTの検査（JSONと `Origin`/`Host`）は `web/internal/server` の `sameOrigin` にある
- エージェントは、開始ラベルを付けた人を `LabelActor` で調べ、`trusted_authors` に入っていれば着手する

## Goals / Non-Goals

**Goals:**

- issueを作る口を、エージェントが使わない別のインターフェースに閉じ込める
- ブラウザから任意のラベルを付けさせない
- WUIに新しい権限を渡さない。WUIは、これまでどおりトークンを持たない

**Non-Goals:**

- WUIの認証（#15）
- ラベル以外の項目（担当者、マイルストーン）の指定

## Decisions

### D1. issueを作る口は `forge.Issues` として `Forge` と分ける

`CreateIssue(ctx, owner, repo, title, body, labels)` だけを持つ。`Hooks`、`Repos` と同じく、受信箱のトークンのクライアントだけが使う。GitHubは `POST /repos/{o}/{r}/issues`、GitLabは `POST /projects/:id/issues`（`labels` はカンマ区切り）で作る。

代わりに `Forge` へ足す案は採らない。エージェントのクライアントにも口ができ、エージェントがissueを作れるように見えるためである。

### D2. ブラウザは開始ラベルの種別だけを送る

要求の `start` は `ready`、`plan`、`go` のどれかで、ラベル名への置き換えは核が設定の `labels` で行う。任意のラベル名を受けると、`agent:done` などの状態ラベルを外から付けられる。設定でラベル名が空の種別は、一覧にも出さず、要求も拒む。

### D3. 1回の要求でラベル付きで作る

作成の要求にラベルを含める。作った人がそのラベルを付けた人として残れば、エージェントの `LabelActor` は受信箱のトークンの持ち主を返し、信頼済みとして着手する。残るかどうかは実物で確かめる（ROADMAP V33）。残らなければ、ラベルなしで作ってから `SetLabels` で付ける2段の形に替える。

### D4. 作れる先は設定のすべてのプロジェクト

`/v1/issue-targets` は `cfg.Projects` を設定の順に返す。`/v1/projects` と違い、担当の端末で絞らない。受信箱のトークンはフォージに1つなので（ADR-031）、ほかの端末の担当プロジェクトにも作れる。`Server.Reload` で `NewIssues.Config` も差し替え、読み直しで現れたプロジェクトを選べるようにする。

### D5. 本文の大きさの上限

本文は1MiBまでとし、復号したあとの長さで比べる。JSONは1バイトを最大6バイトに書くので、HTTPの本文は `6MiB + 64KiB` まで読む。それを超えたら `too_large` を返す。WUIのサーバーも同じ上限で読む。

### D6. ダイアログはネイティブの `<dialog>`

依存を足さず、`showModal` と `close` で開け閉めする。Escapeでの取り消しとフォーカスの閉じ込めはブラウザに任せる。jsdomは `showModal` を持たないので、テストの準備で開け閉めだけを補う。プロジェクトの一覧は、開くたびに読み直す。設定の読み直しで足されたプロジェクトを、ページを開き直さずとも選べるようにするためである。

## Risks / Trade-offs

- LANの中の別の機器から、どのプロジェクトにも実行を起こせる → ADR-040で受け入れる。POSTには返答と同じ検査をかけ、CSRFとDNS rebindingを防ぐ
- 作った人がラベルを付けた人として残らないフォージがある → V33で確かめ、残らなければD3の2段の形に替える
