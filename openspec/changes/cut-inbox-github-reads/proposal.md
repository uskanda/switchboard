# Proposal

## Why

2026-11-14に、Web表示の受信箱が「GitHubを読めませんでした：…403 API rate limit exceeded」の帯だけを出し、行を1つも出さなくなった。Claudeの使用量の上限で実行が始まらず、開始ラベルの付いたissueが87件たまっていた。受信箱のサービスは、`<inbox.refresh_interval>`（30秒）ごとに、その全件について本体、コメント、編集者の3つを読む。1時間にRESTで約2万回となり、GitHubの枠（1時間5000回）の4倍になる。さらに、1件でも読めないとフォージの一覧を丸ごと捨てるので、枠が尽きると画面から行が消える。

## What Changes

- 一覧を作るときは、issueごとにコメントだけを読む。本体と編集者は読まない。GitHubでは、コメントをETagの条件付きリクエストで読み、変化が無ければ304を受けて前の内容を使う
- issueごとの読み取りを、4件ずつ並べて行う
- 読む頻度を、issueごとに変える（`adaptive`）。検索の結果（更新の時刻、人のコメントの数、ラベル）が前回と変わったissueは、すぐ読む。最後に変化を見てから10分以内のissueと、実行中のissueは、`<inbox.refresh_interval>` ごとに読む。それを過ぎたissueは、変化が無いたびに読む間隔を倍にし、`<inbox.idle_interval>`（既定は30分）で止める
- 設定に `<inbox.read_mode>`（`adaptive` か `always`、既定は `adaptive`）と `<inbox.idle_interval>` を足す。`always` は全件を間隔ごとに読む
- issueを読めなかったときは、最後に読めた行を出す。検索ごと失敗したときは、そのフォージの最後に読めた行をすべて出す。上の帯は今までどおり出す。最後に読めた行は、受信箱のサービスのメモリにだけ持つ
- フォージがAPI制限を返したら、リセットの時刻まで、一覧の取り直しではそのフォージを読まない。返答、マージ、1件の詳細などの人の操作は、今までどおり試す
- 一覧の `errors` の要素に、制限中の再開の時刻 `retry_at` を足す。Web表示は、帯に再開の時刻を端末の時刻で出す
- ADR-071を足す。`docs/GUIDE.md` と `docs/GUIDE.ja.md` に、新しい設定を書く

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

- `inbox-api`: 一覧の読み方（読む頻度、読めなかったときの行、API制限中の待ち、`retry_at`）の要件を足し、欠けた一覧のキャッシュと取り直しの間隔の要件を直す
- `config`: `<inbox.read_mode>` と `<inbox.idle_interval>` の要件を足す
- `web`: 帯にAPI制限の再開の時刻を出す要件を足す

## Impact

- `internal/forge`（コメントだけを読むメソッドと、API制限の誤りの型）、`internal/forge/github`、`internal/forge/gitlab`
- `internal/inbox`（一覧での読み方、最後に読めた行、読む頻度）
- `internal/inboxserve`（制限中の待ち、`retry_at`）、`inboxapi`（`ForgeError` の項目）
- `internal/config`（新しい2つの項目）
- `web/ui`（帯の文言、日本語と英語）
- `docs/DECISIONS.md`、`docs/GUIDE.md`、`docs/GUIDE.ja.md`
- CLIの `switchboard inbox` は1回ごとに終わるので、読む頻度を変えず、最後に読めた行も出さない。CLIでも、コメントだけを読み、4件ずつ並べて読む
- Androidのアプリは `errors` を読まないので、変えない
- エージェントの読み取りは変えない
