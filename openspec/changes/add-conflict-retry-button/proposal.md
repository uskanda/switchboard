# Proposal

## Why

baseとの衝突を自動で直す実行が上限の回数に達すると、issueは理由が `conflict` の待ちになる。今のWUIでは、もう一度直させるにも返答の欄に文を書いて送るしかない。理由が `error` の待ちには1回で押せる「再実行する」があるので、衝突の待ちにも同じボタンを設け、手間を減らす。

## What Changes

- 理由が `conflict` の待ちでも、1件の画面に「再実行する」を主のボタンとして置く。押すと、衝突の解消を頼む決まった文 `baseとの衝突を解消して、もう一度pushしてください。` を返答として1回投稿する
- 欄に文があるときと待ちでないときに押せないことは、`error` と同じにする
- 衝突の待ちの案内を、「再実行する」と「返答を送る」の使い分けと、どちらでも自動で直す回数を数え直すことを示す文に変える
- `ci`、`verify`、`permission`、`push` の待ちには、今までどおり「再実行する」を出さない
- `DESIGN.md` と `README.md` の記述を合わせる

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

- `web`：再実行のボタンの対象を、理由が `error` または `conflict` の待ちに広げる。衝突の待ちの案内の要件を足す

## Impact

- `web/ui/src/components/IssuePane.tsx` と `web/ui/src/pages/failure.test.tsx`
- `DESIGN.md`、`README.md`
- サーバー、受信箱のサービス（`inboxapi`）、エージェントは変えない。返答のPOSTをそのまま使う
