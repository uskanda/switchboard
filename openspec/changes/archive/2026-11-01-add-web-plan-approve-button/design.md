# Design

## Context

動機はproposal.mdに、判断はgrilling.mdにある。今の `IssuePane.tsx` は、問いを読み取れた `plan-questions` の待ちで「返答を送る」（副）と「この内容で返答」（主）を並べ、それ以外では「返答を送る」だけを置く。返答は `send()` が `postReply` で送り、送ったあとに「送りました」を出して欄を空にする。承認の語は詳細の `approve_word` にあり、無いときは `/ok` を使う。

## Goals / Non-Goals

**Goals:**

- 計画の承認待ちで、承認の語だけを1クリックで送る
- 直しのつもりで欄に書いた文が、承認として送られないようにする

**Non-Goals:**

- `plan-questions` の待ちと、directの待ちでの承認のボタン（grilling Q1）
- 押したときの確認のダイアログ（grilling Q3）
- 核と `switchboard-web` のサーバーの変更

## Decisions

### 承認のボタンを出す条件

`approving = detail.reason === "plan" && !plan` とする。`plan` は問いを読み取れた質問の画面で、選択が残っている間は理由が変わっても描き続ける。`!plan` を付けて、その画面と重ならないようにする。

### 押せる条件と送る文

承認のボタンは、`sending || !detail.waiting || text.trim() !== ""` のとき押せない。送る文は `word = detail.approve_word || "/ok"` で、`PlanHint` に渡す語と同じ値を1か所で求める。送ったあとの流れは、今の `send()` をそのまま使う。

### 並び

`plan-questions` の画面と同じく、「返答を送る」（`variant="secondary"`）を左に、承認のボタンを右に置く。送っている間は、今のボタンと同じく、承認のボタンの名前を送っている最中の表示に変える。

## Risks / Trade-offs

- 確認を挟まないので、誤って押すと実装が始まる。実装の実行はmainへのpushとマージを拒否され、PRを作るところまでで止まるので、取り返しがつく。案内の文には、承認すると作業が始まることを書いておく
