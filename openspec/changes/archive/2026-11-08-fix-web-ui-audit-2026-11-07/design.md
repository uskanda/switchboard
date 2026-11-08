# Design

## Context

2026-11-07のWeb受信箱の監査で見つかった不具合のうち重い4件と、小さな使い勝手の改善を直す。どれも `web/ui` の中で閉じていて、サーバーとAPIは変えない。

## Goals / Non-Goals

**Goals:**

- 不具合4件を、それぞれ再現するテストを先に書いてから直す
- すぐ直せる使い勝手の改善を入れ、テストで反映を確かめる

**Non-Goals:**

- 監査の残りの不具合（ログインを挟んだディープリンク、スクロールの位置、新規issueのダイアログのEscなど）
- スマホの固定の見出しの縮小や、送信後の仮のカードなど、DESIGN.mdの改訂が要る変更
- スレッドの進捗のカードの整形。今の要件「実行中の表示」は、進捗のコメントの全文を出すよう求めているため、変えない

## Decisions

### 送信の結果は送った先のissueにだけ反映する

`IssuePane` は、別のissueへ移っても同じインスタンスのままである。そのため、`send` のawaitの後の状態の更新が、移った先のissueの表示を書き換えていた。

送信中のissueを `sendingFor`（issueの参照）で持ち、`sending` は `sendingFor === issueRef` で求める。今表示しているissueはrefに入れ、描くたびに更新する。応答が返ったとき、refが送った先と同じなら今までどおり状態を更新する。違えば次のとおりにする。

- 成功：`keepDraft(送った先, 空)` でそのissueの書きかけを消し、`markSent` と `markPending` は今までどおり呼ぶ
- 失敗：`toast.error` で知らせる。書きかけは `keepDraft` に残っているので、何もしない

送信中に別のissueへ移ったとき、元のissueの書きかけは描画の時点で `keepDraft` に保存されている。そのため、成功時に消す処理は、送った文と同じかを確かめずに書きかけを消してよい。

### 401 `unauthenticated` はログインを有効と見なす

`AuthGate` の `ended` と、`readStatus` の結果をセッション切れと合わせる箇所で、`enabled: true` を明示する。`/api/auth` を再試行する案もあるが、401はログインが有効であることの証拠なので、状態を1か所で上書きすれば足りる。

### ログアウトは応答を確かめる

`res.ok` でないときと通信の失敗では、`toast.error` で知らせ、状態を変えない。成功したときだけ `user` を消す。

### マージのボタンの `later` は、一覧に `auto_merge` が載ったら消す

`later` は、マージの応答から一覧が追いつくまでの間だけ使う。`autoMerge` が真になったら `setLater(null)` し、以後は `autoMerge` だけでボタンの状態を決める。一覧が載る前に取り消された場合は、`later` が残る。この場合は、今と同じく開き直すまで押せない。この隙間は短いため、受け入れる。

### 経過時間は共有の1分の時計で描き直す

`lib/now.ts` に `useMinute()` を置く。1つの `setInterval`（60秒）を `useSyncExternalStore` で購読させ、使う部品だけを描き直す。`InboxList`、`IssuePane`、`InboxLayout` の何も選んでいない画面、`Header` で呼ぶ。テストでは偽のタイマーで時計を進める。

### `cn` にデザイントークンを登録する

`twMerge` は `p-lg`、`text-label-md`、`rounded-md` などの独自のトークンを知らない。そのため、`p-lg` と `p-0` を同じ種類の指定として扱わず、カードの余白が二重になっていた。`extendTailwindMerge` で、`tokens.css` のトークンを登録する。登録するのは、spacing（xs〜xl）、text（headline-md、body-md、label-md、mono-sm）、radius（sm、md、full）である。登録すると、`text-label-md` を色の指定と見なして `text-muted` で消す誤りも起きなくなる。

### 完了の注記は節の見出しの下へ

`cells` が完了の行に返す `line` を空にし、空の `line` は描かない。注記は、完了の節の見出しの下に `text-label-md text-muted` の文として1回だけ出す。

## Risks / Trade-offs

- `cn` の修正は、すべての部品のクラスの統合を変える → 今まで残っていた指定のうち、後の指定と重なるものは消える。そのため、見た目の変わる箇所が出うる。既存のテストと、主な画面のスクリーンショットで確かめる
