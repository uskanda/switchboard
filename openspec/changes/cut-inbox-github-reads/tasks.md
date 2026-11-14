# Tasks

## 1. 実物の挙動の確認

- [x] 1.1 GitHubのissueのコメントの一覧を、前回の `ETag` を `If-None-Match` に付けて求める。304が返ることと、`x-ratelimit-used` が増えないことを確かめる。結果を `docs/ROADMAP.md` の検証項目にV60として書く
- [x] 1.2 API制限の応答のヘッダを、文書か実物の応答で確かめる。GitHubは403と429の `x-ratelimit-remaining`、`x-ratelimit-reset`、`retry-after` を見る。GitLabは429の `RateLimit-Reset` と `Retry-After` を見る。結果をV61として書く
- [x] 1.3 GitHubで既存のコメントを書き換え、検索の結果の `updated_at` と `comments` が変わるかを確かめる。結果をV62として書く。変わらなくても設計は変えない（design.mdのOpen Questions）

## 2. 設定

- [x] 2.1 `internal/config` の失敗するテストを書く。`<inbox.read_mode>` と `<inbox.idle_interval>` の既定（`adaptive`、30分）を確かめる。知らない値と、`<inbox.refresh_interval>` より短い `<inbox.idle_interval>` が拒まれることも確かめる
- [x] 2.2 `config.Inbox` に `ReadMode` と `IdleInterval` を足し、既定と検証を入れる。2.1のテストが通ることを確かめる
- [x] 2.3 `config/` の設定の例に、2つの項目をコメントで足す。`docs/GUIDE.md` と `docs/GUIDE.ja.md` の受信箱の設定の節にも、同じ説明を書く。設定の例の検査（`make verify`）が通ることを確かめる

## 3. フォージのクライアント

- [x] 3.1 GitHubのクライアントの失敗するテストを、テスト用のHTTPサーバーで書く。`Comments` がコメントだけを全ページ読むことを確かめる。2回目は `If-None-Match` を付け、304なら前回の結果を返すことも確かめる。検索の `comments` が `IssueSummary` の `Notes` に入ることも確かめる
- [x] 3.2 GitHubのクライアントの、API制限の失敗するテストを書く。`x-ratelimit-remaining: 0` の403、`retry-after` のある403、ヘッダの無い429を使う。それぞれがResetを持つ `<forge.RateLimitError>` になることを確かめる。403では `errors.Is(err, forge.ErrForbidden)` が真のままであることも確かめる
- [x] 3.3 `forge.Forge` に `Comments` を、`forge` に `<forge.RateLimitError>` を足す。GitHubのクライアントに実装し、`Notes` の説明を直す。3.1と3.2のテストが通ることを確かめる
- [x] 3.4 GitLabのクライアントの失敗するテストを書く。`Comments` がシステムのノートを除くこと、429が `<forge.RateLimitError>` になることを確かめる。実装してテストが通ることを確かめる
- [x] 3.5 `Comments` を、テスト用の偽のフォージのすべてに足す。`go build ./...` と `go test ./...` が通ることを確かめる

## 4. 一覧の読み方

- [x] 4.1 `internal/inbox` に、一覧の読み方の失敗するテストを書く。呼び出しを数える偽のフォージを使う。一覧が `Thread` を呼ばず、`Comments` だけを呼ぶことを確かめる。同時に読むのが4件までであることも確かめる
- [x] 4.2 `list` を `Comments` に替え、4件ずつ並べて読む。4.1のテストと、既存の一覧のテストが通ることを確かめる
- [x] 4.3 `internal/inbox` に、読む頻度の失敗するテストを書く。時計は差し替える。inbox-apiの「一覧での読む頻度」のシナリオをすべて確かめる。読んだコメントが前回と違うときに、間隔が戻ることも確かめる
- [x] 4.4 `Inbox` にissueごとの記録を足し、4.3のテストが通ることを確かめる。検索に無くなったissueの記録が捨てられることも確かめる
- [x] 4.5 `internal/inbox` に、読めなかったときの失敗するテストを書く。場合は、1件のコメントの失敗、検索の失敗、起動の直後の検索の失敗の3つとする。inbox-apiの「読めなかったissueの行」のとおりになることを確かめる。実装してテストが通ることを確かめる
- [x] 4.6 API制限の失敗するテストを書く。制限を受けたあと、Resetまでの `Collect` はそのフォージに要求を送らず、記録の行と再開の時刻のある誤りを返す。Resetのあとは読みに行く。`Detail` と `Reply` は制限中もフォージを呼ぶ。これらを確かめ、`inbox.ForgeError` に `RetryAt` を足して実装し、テストが通ることを確かめる

## 5. 受信箱のAPI

- [x] 5.1 `internal/inboxserve` に、受信箱のAPIの失敗するテストを書く。`RetryAt` のある誤りだけが、一覧の `errors` に `retry_at` を持つことを確かめる。制限中の一覧をキャッシュせず、次の要求でもフォージに要求が行かないことも確かめる
- [x] 5.2 `inboxapi.ForgeError` に `RetryAt` を足し、`fetchLocked` で写す。5.1のテストが通ることを確かめる。`web/ui/src/lib/api.ts` の型にも `retry_at` を足す

## 6. Web表示

- [x] 6.1 Web表示の帯の失敗するテストを書く。`retry_at` のある誤りの帯は、API制限中であることと、ブラウザの時間帯の時と分を出す。`retry_at` の無い誤りの帯は今までどおりである。これを日本語と英語で確かめる
- [x] 6.2 `InboxErrors` に、制限の文言を `defineMessages` で日本語と英語で足す。6.1のテストが通ることを確かめる

## 7. 決定の記録と確認

- [x] 7.1 `docs/DECISIONS.md` にADR-071を足す。読む頻度の決め方、読めなかったときの行、API制限中の待ちを書く。inbox-apiの「変化の知らせ」の例外も書く
- [x] 7.2 `make verify` を通す
- [ ] 7.3 `switchboard inbox serve` を実物の設定で起動し、Web表示を開いたまま30分おく。`gh api rate_limit` で、coreの使用数の増え方を測る。直す前は1時間に約2万回だった。枠に収まる数まで減ったことを確かめ、結果をV63として `docs/ROADMAP.md` に書く
