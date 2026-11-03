# Design

## Context

動機はproposal.mdのWhyを参照。決めたことはgrilling.mdに記録してある。今の受信箱のサービスは状態を持たず、フォージから毎回組み立て直している。マージ（ADR-043）は、押したときに一度だけフォージへ送り、断られたら理由を返して終わる。衝突は、エージェントのCIの見張り（ADR-044）が見つけて直す。

## Goals / Non-Goals

**Goals:**

- 衝突で断られたマージを、直す実行が新しいdoneを投稿したあと、人の身元（受信箱のトークン）でマージする
- マージするコミットを、押したあとに直す実行がpushしたものに絞る
- 受信箱のサービスのラベルとコメントに触らない約束（ADR-043）を保つ

**Non-Goals:**

- フォージのauto-merge（GitHubの `enablePullRequestAutoMerge`）は使わない
- 受信箱のサービスはCIを読まない。CIの待ちはフォージの断りで知る
- 衝突でない理由で断られたマージを、自動では試し直さない
- コマンドラインの `switchboard inbox` には、自動マージを出さない

## Decisions

### doneのマーカーに `fix` を足す

`marker.Marker` に `Fix` を足し、`fix=<ci|push|conflict>` として書く。`reason` は待ちの理由と着手の `fix` に使っているので、別のキーにした。エージェントでは、`outcome` に `Fix` を持たせ、`finish` で直す実行（`pl.fix != nil`）のときに原因を入れる。`writeOut` は、doneのときだけマーカーに `Fix` を書く。原因の値（`prompt.FixCI` など）は、マーカーの値と同じ文字列なので写すだけで済む。`outcome.json` にも残るので、書き戻しの途中で止まっても印は失われない。

### `forge.Merges` に `ChangeRequestHead` を足す

衝突しているかとheadは、CIの見張りが使う `ChangeRequestHead` で読める。GitHubとGitLabのクライアントは既に実装しているので、`Merges` のインターフェースに足すだけでよい。受信箱のトークンの権限は、ADR-043で足した分を超えない。fakeでは、`SetChange` で登録したPR/MRには、その状態を返す。`Change` に `Conflict` と `Closed` を足す。

### 依頼の置き場は `internal/automerge`

`Store`（`internal/automerge`）が `inbox/automerge.json` を読み書きする。中身は、待っている依頼（`pending`）と、終わった依頼（`finished`）の2つの配列である。書くたびに `WriteJSON`（`state.Dir`）で一時ファイルからrenameする。終わった依頼は24時間残し、読むときと書くときに24時間より古いものを消す。終わらせるときはrefと受け付けた時刻で照らし、見張りの最中に押し直されて置き換わった依頼を、古い結果で終わらせない。

### 判断は `internal/inbox` に置く

依頼の登録の材料（最新のdoneのマーカー、最後のコメントID、PR/MRの状態）と、見張りの判断は、どれもissueのスレッドとエージェントの身元を読む。これらは `inbox.Inbox` が持っているので、`AutoMergeCandidate` と `AutoMergeStep` を `inbox` に置く。信頼済みの人の判定には、`trusted_authors` のログインをフォージで数値のIDに直して使う。エージェントの身元と同じように覚えておく。判定は打ち切りの側にだけ使うので、編集の有無は見ない。人の指示を見落とすより、余計に打ち切るほうが安全である。

コメントの前後はコメントIDで決める。エージェントの着手の勝ち負けも、フォージが振るIDで決めている（`claim`）。

### 見張りは `RunAutoMerge`（`inboxserve.Server`）

画面の購読の `loop` とは別のゴルーチンで、`switchboard inbox serve` の起動から終了まで動く。タイマーは `Interval` ごとに鳴るが、依頼が無ければ何もしない。マージの受け付けと `RelayEvent` は、バッファ1のチャネルで見張りを起こす。依頼が終わったら、キャッシュを捨てて `readAgain` を呼ぶ。一覧を組み立てるときに `Store` から `auto_merge` と `auto_merge_results` を足す。購読者へ送るかを決める比較にも `auto_merge_results` を含める。

### APIの形

- `POST /v1/merge` は、依頼を登録したら202と、`code` が `auto_merge_pending` のJSONを返す。クライアントの `Merge` は `*MergeResult`（`AutoMerge bool`）を返す
- `POST /v1/automerge/cancel` は `{ref}` を受け、204か404を返す
- `Item` と `Detail` に `auto_merge`（`sha`、`since`）、`Inbox` に `auto_merge_results` を足す。`inbox.Item` からの型の変換は、項目を写す関数に置き換える

### WUI

`MergeButton` は、202を受けたらトーストを出し、自分の状態を「自動マージ待ち」にする。一覧が読み直されると `auto_merge` が付き、印とボタンの無効化は項目の値から出る。取り消しのボタンは詳細のペインにだけ置き、確認のダイアログを経て送る。終わった依頼のトーストは、受信箱の購読を持つ部品で出し、見せた件を `localStorage` に覚える。

## Risks / Trade-offs

- ［GitHubで405のあと、`mergeable` がまだ計算中で `Conflict=false` になる］→ そのときは今までどおり409を返す。人が押し直す。実物の確認はROADMAP.mdの検証項目に残す
- ［ブランチの保護でCIを必須にしていないリポジトリでは、直す実行のdoneのすぐあとに、CIを待たずにマージされる］→ 人が押すマージも同じである。CIの必須化はブランチの保護に任せるという、ADR-043の分担を保つ
- ［見張りは依頼ごとにissueとPR/MRを読む］→ 依頼は人が押した分だけで、数は少ない。間隔は `inbox.refresh_interval`（10秒以上）に従う
