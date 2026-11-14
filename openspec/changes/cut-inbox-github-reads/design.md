# Design

## Context

動機はproposal.mdのWhyにある。設計を決める今の状態は次のとおり。

- 一覧は `internal/inbox` の `Collect` が作る。フォージごとに `list` を並べて動かし、`list` は検索（`SearchOpenIssuesWithLabel`）のあと、見つかったissueを1件ずつ `Thread` で読む。
- `Thread` は、GitHubでは本体、コメント、編集した人（GraphQL）の3つを、GitLabでは本体、ノート、編集の記録の3つを読む。一覧が使うのはコメントの本文、作者、作成の時刻だけである。
- `list` は、1件でも誤りがあると `nil` を返し、そのフォージの行はすべて消える。
- 受信箱のサービス（`internal/inboxserve`）は、1つの `Inbox` を持ち続け、`fetchLocked` から `Collect` を呼ぶ。CLIの `switchboard inbox` は、呼ぶたびに `Inbox` を作る。
- GitHubのクライアントは、`UpdatedIssues` で既にETagの条件付きリクエストを使っている。304は枠に数えられない。
- 検索の結果（`forge.IssueSummary`）は、ラベルと `UpdatedAt` を持つ。`Notes` はGitLabの `user_notes_count` だけで、GitHubでは0である。
- フォージのクライアントは、API制限の応答を区別していない。403は `forge.ErrForbidden` になる。

## Goals / Non-Goals

**Goals:**

- 開始待ちのissueが100件近くあっても、受信箱のサービスの一覧が、GitHubの1時間5000回の枠を使い切らない。
- 枠を使い切っても、画面から行が消えない。

**Non-Goals:**

- エージェントの読み取りを減らすこと（grilling.mdの後回し）。
- 古い行に印を付けること、Androidのアプリで再開の時刻を出すこと。
- 1件の詳細の読み取りを減らすこと。人が開いたときだけ読むので、枠への影響は小さい。

## Decisions

### コメントだけを読むメソッドを、フォージのインターフェースに足す

`forge.Forge` に `Comments(ctx, ref) ([]Comment, error)` を足す。GitHubは `/issues/{n}/comments` の全ページ、GitLabは `/notes` の全ページからシステムのノートを除いたものを返す。`Comment` の `EditedBy` は埋めない。一覧はそれを使わない。

`Thread` に「本体を読まない」選択肢を足す案は採らない。`Thread` の呼び手は十数か所あり、引数を変えるとすべてに手が入る。

GitHubの `Comments` は、最初のページのURLをキーに、ETagと前回の結果をクライアントのメモリに持つ。最初のページが304なら前回の結果を返し、続くページは読まない。最初のページが変わったときだけ、続くページを読む。100件を超えるコメントのあるissueで、2ページ目だけが変わることは、最初のページのETagに出ないことがある。だが、そのissueでも、検索の `comments` の数と `updated_at` の変化で読み直しは起きる。最初のページのETagが同じでも、件数が変わっていれば全ページを読む。

### 4件ずつ並べて読む

`list` の中で、読むと決めたissueを、4件ずつ並べて `Comments` で読む。GitHubの二次的な制限（同時の要求の数）にかからない数として、4にする。並べる前に、`agentID` をプロジェクトごとに解決しておく。

### 読む頻度の記録は `Inbox` のメモリに持つ

`Inbox` に、issueごとの記録を持たせる。

- 前回の検索での手がかり：`UpdatedAt`、コメントの数、ラベルの組
- 最後に変化を見た時刻
- 最後に読んだ時刻と、読む間隔
- 最後に読めた行（`Item`）と、そのときのコメント

GitHubでは、検索の結果の `comments` を `IssueSummary` の `Notes` に入れる。GitHubで `Notes` を0とする今の決まりは、エージェントの返答の検知（GitLabだけ）にしか使われていない。GitHubで入れても、そこは変わらない。`Notes` の説明を直す。

取り直しのたびに、`Inbox` は次の順で決める。

1. 検索の結果の手がかりが記録と違う、記録に無い、実行中、最後の変化から10分以内のどれかなら読む。手がかりが違えば、最後に変化を見た時刻を今にし、読む間隔を `<inbox.refresh_interval>` に戻す。
2. それ以外は、最後に読んだ時刻から読む間隔がたっていれば読み、読む間隔を倍にする（上限は `<inbox.idle_interval>`）。
3. 読まないissueは、記録のコメントから行を作り直す。行は検索の結果（タイトル、ラベル、更新の時刻）とコメントから作るので、コメントを読まなくても、ラベルとタイトルの変化は行に出る。

読んで得たコメントが記録のコメントと違うときも、変化を見たとみなし、1と同じに戻す。検索の手がかりに出ない変化（コメントの書き換え）を見つけたときに、頻度を戻すためである。

検索の結果に無くなったissueの記録は捨てる。`Inbox` は `Reload` で作り直されるので、設定を読み直すと記録も消え、全件を一度読む。

CLIでは `Inbox` が1回ごとに作られるので、記録は空で、全件を読む。`always` も、手順の2と3を飛ばして全件を読む。

10分という境と、倍にする元を `<inbox.refresh_interval>` にすることは、grilling.mdのQ9とQ10で決めた。10分は定数にする。

### 読めなかったときは記録の行を使う

`Comments` が失敗したissueは、記録の行（無ければ出さない）を使い、フォージの誤りを1つ返す。検索が失敗したフォージは、そのフォージの記録の行をすべて使う。記録は捨てない。誤りの文は、最初に失敗したissueの誤りとする。

### API制限は、フォージのクライアントが型で返す

`forge` に `RateLimitError{Reset time.Time; Err error}` を足す。`Unwrap` は今の誤り（GitHubの403なら `ErrForbidden`）を返す。今の `errors.Is(err, forge.ErrForbidden)` の判定は変わらない。

- GitHub：403か429で、`x-ratelimit-remaining` が `0` なら `x-ratelimit-reset`（UNIXの秒）をResetにする。`retry-after` があれば、今の時刻にその秒数を足してResetにする（二次的な制限）。
- GitLab：429で、`RateLimit-Reset` か `Retry-After` からResetを作る。

どちらも、ヘッダが無いときは今から60秒後をResetにする。

`Inbox` はフォージごとに「Resetまで読まない」時刻を持つ。`Collect` はその時刻までそのフォージに要求を送らず、記録の行と、`ForgeError{RetryAt: Reset}` を返す。制限の判定を `Inbox` の一覧の経路だけに置くので、1件の詳細、返答、マージは今までどおりフォージに要求を送る。そこで制限を受けたときも、その誤りは一覧の待ちに足さない。人の操作の結果で一覧を止めると、制限の解けたあとも行が古いまま残りうるためである。

`inboxapi.ForgeError` に `RetryAt *time.Time` を `retry_at`（省略可）として足す。`inbox.ForgeError` にも同じ項目を足し、`inboxserve` が写す。

### 欠けた一覧のキャッシュ

`fetchLocked` が欠けた一覧をキャッシュしない今の動きは変えない。APIの要求ごとに `Collect` を呼んでも、制限中のフォージには `Inbox` が要求を送らない。読む頻度の記録で、読まないissueはコメントを読まない。そのため、キャッシュしなくても枠は減らない。

### Web表示の帯

`InboxErrors` の帯で、`retry_at` があればAPI制限中であることと、読み直す時刻を出す。時刻は `Intl.DateTimeFormat` で、ブラウザの時間帯の時と分にする。文言は `defineMessages` に日本語と英語を書く。

## Risks / Trade-offs

- リスク：検索の結果に出ない変化は、読む間隔が延びたissueでは最大 `<inbox.idle_interval>` 遅れて画面に出る。対策：GitHubでは、コメントの投稿、書き換え、削除のどれでも検索の `updated_at` が動くことをV62で確かめた。GitLabでは、コメントの数とラベルの変化を手がかりにし、読んだコメントが前回と違えば頻度を戻す。
- リスク：メモリの記録が、issueの数だけ増える。対策：検索の結果に無くなったissueは捨てる。100件で数百KBに収まる。
- リスク：GitHubが、コメントの一覧のETagで304を返さない。対策：V60で、304が返り枠を使わないことを確かめた。返さなくなっても、読む頻度の仕組みだけで、87件の読み取りは1時間あたり約200回になる。
- リスク：制限のヘッダの名前と形が想定と違う。対策：ヘッダが無いときは60秒待つ。実物の応答をタスクで確かめる。
- リスク：サービスを起動し直した直後と、設定の読み直しの直後は、全件を読む。対策：87件で1回174回の要求であり、枠に収まる。

## Migration Plan

設定を足すだけで、既定の `adaptive` は受信箱のサービスの再起動から働く。戻すときは、`<inbox.read_mode>` を `always` にする。`inboxapi` の `retry_at` は省略可の項目で、古いクライアント（Androidのアプリ）は読み捨てる。

## Open Questions

なし。GitHubの検索の `updated_at` がコメントの書き換えで動くかは、V62で確かめた。動くので、書き換えの遅れは次の取り直しまでで済む。
