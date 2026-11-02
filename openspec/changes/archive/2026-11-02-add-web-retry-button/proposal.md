# Proposal

## Why

実行がタイムアウトや中断で失敗すると、issueは理由 `error` の待ちになる。再実行するには、受信箱のWeb表示で返答の欄に何か書いてから「返答を送る」を押す必要があり、同じ作業をやり直すだけでも文を考えて打つ手間がかかる。1クリックで再実行を頼めるようにする（uskanda/switchboard#72）。

## What Changes

- 理由が `error` の待ちの1件の画面に、「再実行する」のボタンを主のボタンとして置く。押すと、再実行を頼む決まった文を今の返答のPOSTで投稿する
- 返答の欄に空白以外の文があるとき、または待ちでないときは、このボタンを押せなくする
- 「返答を送る」は副のボタンにして、「再実行する」の左に並べる
- 理由が `error` の待ちでは、返答の欄の上に、2つのボタンの使い分けを示す案内を1行出す
- ほかの失敗の理由（`verify`、`permission`、`push`、`ci`）には、このボタンを出さない
- DESIGN.mdとREADME.mdに、失敗の待ちの案内とボタンを書き足す

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- `web`: 実行の失敗の待ちの表示に、再実行のボタンと案内を足す

## Impact

- `web/ui`：`IssuePane.tsx` のボタンと案内の文、`failure.test.tsx` のテスト
- DESIGN.mdの1件の画面の項と、README.mdのWeb表示の説明
- 核、`inboxapi`、`switchboard-web` のサーバー、エージェントは変えない。理由が `error` の待ちは、信頼済みのコメントが付けば再実行が始まる。ボタンが送るのはそのコメントである
- 不変条件に触れる変更はないため、ADRは足さない
