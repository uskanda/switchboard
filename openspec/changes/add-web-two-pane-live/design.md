# Design

## Context

動機はproposal.mdに、判断はgrilling.mdにある。設計の前提になる今の形は、次のとおりである。

- `switchboard inbox serve`（`internal/inboxserve`）は、要求が来たときだけ `Collect` でフォージに問い合わせ、完全な一覧を60秒キャッシュする。返答を投稿したらキャッシュを捨てる
- `Collect` は、検索1回（GitLabはプロジェクトごと）のあと、issueごとに `Thread` で本文とコメントを取る。一覧の要素は、取ったコメントから作る
- `inbox.Detail` も `Thread` を取るが、返すのはエージェントの最新の投稿だけである
- `switchboard-web`（`web/internal/server`）は、`inboxapi.Client` で要求ごとに受信箱のサービスを呼ぶ。`http.Server` の `WriteTimeout` は60秒である
- 画面は `react-router` で `/`、`/issue/:ref`、`/settings` を出し分ける。どれも `Shell` の幅720pxの1列に入る

## Goals / Non-Goals

**Goals:**

- 画面を開いている間は、一覧と選んだissueの変化が、再読み込みしなくても間隔（既定30秒）以内に画面へ出る
- 1件の表示で、依頼の本文から最新の投稿までを読んで返答できる
- 画面を開いていない間、フォージへの問い合わせの回数は今と変わらない

**Non-Goals:**

- 中継のwebhookのイベントによる即時の取り直し（issueに残す）
- 待ちと実行中以外のissueの表示
- 返答以外の操作（ラベルの付け替え、コメントの編集）

## Decisions

### 1. 一覧の要素と1件の詳細の型

`inbox.Item`（と、その型を写した `inboxapi.Item`）に、次の2つを足す。

- `updated_at`：`IssueSummary` の `UpdatedAt`
- `notes`：`IssueSummary` の `Notes`（GitLabの `user_notes_count`。GitHubでは0）

`switchboard inbox --json` も同じ項目を出す。`inbox-api` の「一覧」の要件は `switchboard inbox --json` と同じ項目を求めているので、両方に足す。

`inbox.Detail`（と `inboxapi.Detail`）に、次を足す。

- `author`、`created_at`、`body`：issueの書いた人、作った時刻、本文
- `comments`：`Thread` のコメントを古い順に並べたもの。各要素は `author`、`created_at`、`body`、`agent`、`kind` を持つ

`agent` は、書いた人がエージェントの身元で、本文の最後の行がマーカーとして読めるときに真とする。そのとき `kind` はマーカーの状態（`running`、`progress`、`waiting`、`done`）にし、本文からマーカーの行を除く。人のコメントの本文はそのまま返す。`Thread` は既に取っているので、フォージへの問い合わせは増えない。

### 2. 問い合わせのループと `/v1/events`

`inboxserve.Server` に `<Interval>` を足し、`<v1/events>` を出す。

- 接続が来たら、購読者として登録する。購読者が0人から1人に増えたら、ループのゴルーチンを起こす。購読者が0に戻ったら、ループを止める
- 接続の直後に、そのときの一覧を1件目の知らせとして送る。キャッシュが新しければキャッシュを、無ければその場で取った一覧を送る
- ループは `<Interval>` ごとに `Collect` を呼ぶ。取った一覧を、前の一覧と `FetchedAt` を除いて比べ、違えばすべての購読者に送る
- 返答を投稿したら、キャッシュを捨て、ループが動いていれば、間隔を待たずに取り直させる
- 取った一覧は、届かなかったフォージが無ければ、GETの `/v1/inbox` のキャッシュにも入れる。ループが動いている間は、キャッシュがいつも間隔より新しい

知らせの形は、次のとおりである。

```
event: inbox
data: {"items":[...],"errors":[...],"fetched_at":"..."}

```

切れた接続を見つけるため、15秒ごとにSSEのコメント行（`: ping`）を送る。送るのに失敗したら、その購読者を外す。

購読者ごとの送り先は、長さ1のチャネルにする。送るときにチャネルが埋まっていれば、古い一覧を捨てて新しい一覧を入れる。遅い購読者がループを止めないためである。

### 3. `inboxapi` のクライアント

`inboxapi.Client` に、メソッド `<Events>(ctx, func(*Inbox)) error` を足す。`/v1/events` につなぎ、`event: inbox` の `data` を読むたびに関数を呼ぶ。接続が切れたら誤りを返し、つなぎ直しは呼んだ側に任せる。

### 4. `switchboard-web` の `/api/events`

- ブラウザの接続ごとに、`inboxapi.Client` の `<Events>` で受信箱のサービスにつなぎ、届いた一覧を `event: inbox` としてブラウザへ送る
- `http.ResponseController` で、この応答だけ書き込みの期限を外す。`WriteTimeout` の60秒で切られないためである
- 受信箱のサービスに届かないときは、200で `event: unavailable` を1件送って閉じる。`EventSource` は、200以外の応答ではつなぎ直さないためである。応答の始めに `retry: 5000` を送り、つなぎ直しの間隔を5秒にする
- GETだけを受け、`Origin` は検査しない。読むだけで、今の `/api/inbox` と同じ扱いである。CSPの `connect-src` は `default-src 'self'` に含まれる

### 5. 画面の組み方

- `Shell` を作り直す。ヘッダは全幅で、名前、待ちと実行中の件数、自動更新の状態、設定へのリンクを並べる
- `/` と `/issue/:ref` は、同じ `InboxLayout` で描く。`lg`（1024px）以上では、左に一覧（幅360px、縦にスクロール）、右に1件を並べる。`lg` 未満では、`/` で一覧だけを、`/issue/:ref` で1件だけを出し、1件の上に一覧へ戻るリンクを置く
- `/` の右のペインには、「左の一覧からissueを選んでください」と出す。自動では選ばない
- 一覧の状態（最新の一覧、接続の状態）は、`InboxLayout` の1つのフックで持つ。フックは `EventSource('/api/events')` を開き、`inbox` の知らせで一覧を書き換え、`unavailable` と `onerror` で接続の状態を「再接続中」にする
- 設定の画面は、同じヘッダの下に、今の幅720pxの1列で出す

### 6. 1件の表示の追随

- 1件の表示は、一覧の中の選んだissueの `updated_at`、`notes`、`state`、`reason`、`progress` を並べた鍵を見る。鍵が変わったら `/api/issue` を取り直す。最初の一覧が届く前は、鍵が無いものとして1回だけ取る
- 選んだissueが一覧から消えたときは、最後に取った表示を残し、待ちでも実行中でもなくなったことを示す。完了、閉じた、ラベルが外れた、のどれでも同じである
- 返答の欄の文字列は、1件の表示の状態とは別に持つ。取り直しても消えない。取り直した結果が待ちでなければ、送信のボタンを押せなくして、その旨を出す
- `ref` が変わったら、返答の欄を空にする

### 7. スレッドの描き方

- 本文とコメントは、`react-markdown` と `remark-gfm` で描く。`rehype-raw` は使わない。このとき生のHTMLは文字として出る
- `components` で、`img` を「画像：<alt>」のリンクに置き換える。`a` には `target="_blank"` と `rel="noreferrer"` を付ける
- エージェントの投稿には「エージェント」の印を付ける。`kind` が `running` のものは、1行（「着手しました」と時刻）に畳む。`progress` のものは、最も新しい1件だけを出す
- 最後の `waiting` の投稿（待ちの質問）は、枠で囲んで目立たせる
- 拒否された操作の一覧と、計画の承認の案内は、今と同じく返答の欄の近くに出す

### 8. 設定

`config.Config` に `Inbox` の節を足し、`<refresh_interval>` を `time.Duration` として読む。無ければ30秒にする。10秒未満は、設定の検証で拒む。`switchboard inbox serve` は、この値を `inboxserve.Server` の `<Interval>` に渡す。

## Risks / Trade-offs

- ［画面を開いている間、フォージへの問い合わせが増える］→購読者がいる間だけループを動かす。待ちが5件で30秒なら、REST APIは約600回/時、検索は120回/時である。GitHubの検索の上限（30回/分）とRESTの上限（5,000回/時）に収まる
- ［ブラウザのタブを開いたままにすると、問い合わせが続く］→意図した動きである。間隔は `refresh_interval` で延ばせる
- ［人のコメントの生のHTML（issueのテンプレートのHTMLのコメントなど）が文字として出る］→grillingで決めたとおりである。見づらければ、HTMLのコメントだけを消す扱いを別に決める
- ［Markdownのライブラリで、画面のJavaScriptが大きくなる］→gzipで数十KB増える。LANの中で使う画面なので受け入れる

## Migration Plan

- 古い `switchboard inbox serve` には `/v1/events` が無い。新しい `switchboard-web` は、404を受けたら `event: unavailable` を送る。核とWUIは同時に置き直す
- `config.yaml` に `inbox` の節が無くても動く
