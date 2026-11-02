# Proposal

## Why

WUIで返答を送ると、送ったことの表示（「送りました」）が出る。そのあとエージェントが次の待ちを投稿しても、画面を読み込み直すまで、この表示が残る。返答の欄も出ないため、次の待ちに答えられない（#52）。

原因は、`IssuePane` が返答のあとに立てる印を、別のissueを開くまで戻さないことである。issueを取り直しても、返答の欄と状態の文言は隠れたままになる。

## What Changes

- 返答を送った時点の待ちを、待ちの開始時刻とエージェントの最後の待ちのコメントで覚える
- 取り直したissueがその待ちでなくなったら、「送りました」を消し、その時点の状態を表示する。待ちでなくなるのは、実行中、順番待ち、新しい待ちになったときと、一覧から消えたときである。新しい待ちなら返答の欄を出す
- 同じ待ちが返ってくる間は「送りました」を残す

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- `web`: 自動更新に、返答のあとの表示の戻し方を足す

## Impact

- `web/ui/src/components/IssuePane.tsx`
- `web/ui/src/pages/live.test.tsx`、`web/ui/src/pages/pages.test.tsx`
