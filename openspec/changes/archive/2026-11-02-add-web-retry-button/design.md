# Design

## Context

動機はproposal.mdに、判断はgrilling.mdにある。理由が `error` の待ちは、最後の投稿より後に信頼済みのコメントが付けば、エージェントが前の実行を引き継いで再実行する。`IssuePane.tsx` には、計画の承認待ちで承認の語だけを1クリックで送る作り（`approving`）が既にある。返答は `send()` が `postReply` で送り、送ったあとに「送りました」を出して欄を空にする。

## Goals / Non-Goals

**Goals:**

- 理由が `error` の待ちで、再実行を頼む決まった文を1クリックで送る
- 直し方のつもりで欄に書いた文を、ボタンで捨てたり無視したりしない

**Non-Goals:**

- 核、`inboxapi`、`switchboard-web` のサーバー、エージェントの変更（grilling Q2）
- `verify`、`permission`、`push`、`ci` の待ちでの再実行のボタン（grilling Q1）
- 押したときの確認のダイアログ

## Decisions

### 再実行の経路

再実行の専用APIは作らず、決まった文を今の返答のPOSTで送る。信頼済みの投稿者の検証と、待ちでないときの `not_waiting` の拒否が、今の返答の経路でそのまま働くためである。専用APIを足して `comments.language` に合わせた文をサービスで作る案（grilling Q2のB）は、CLIから再実行したくなったときに足す。

### 送る文

`RETRY_TEXT` という定数に `再実行してください。` を置く。このコメントはClaudeへの指示としても読まれる。そこで、前回のセッションを素直に再開できるよう、指示を足さない短い文にする（grilling Q2のCを採らない）。

### ボタンを出す条件と押せる条件

`retrying = detail.reason === "error" && !plan` とし、`approving` と同じ形で分岐を足す。「再実行する」（`type=button`）は `sending || !detail.waiting || text.trim() !== ""` のとき押せない。「返答を送る」（`variant="secondary"`、`type=submit`）は、送っている間、待ちでない間、欄が空の間は押せない。並びと送っている間の表示は、承認のボタンと同じにする（grilling Q3）。

### 案内

`PlanHint` に `reason === "error"` の枝を足し、2つのボタンの使い分けを1行で出す。DESIGN.mdには、この案内とボタンの色（主は `button-primary`、副は `button-secondary`）を書き足す。

## Risks / Trade-offs

- 確認を挟まないので、誤って押すと同じ作業の再実行が始まる。実行はmainへのpushとマージを拒否されるため、取り返しがつく
- 送る文は日本語で固定であり、`comments.language` を英語にした環境でも日本語のコメントが付く。後回しにしたもの（grilling.md）に記録した
