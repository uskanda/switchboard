# Proposal

## Why

同じ画面で、一覧があるissueを「順番待ち」と出し、1件の画面が「実行中」と出すことがあった（issue #82）。一覧は受信箱のサービスが一定の間隔で読んだ結果で、1件は開いた時点でフォージから読み直した結果だからである。どちらを信じればよいか分からない。

## What Changes

- 受信箱のサービスは、`/v1/issue`で読んだ1件の状態が最後に配った一覧と違えば、一覧をすぐ読み直す。新しい一覧はSSEで届く
- Web UIの1件の画面は、1件の状態が一覧の行と違うあいだ、状態の横に「フォージから今読んだ状態です。一覧は更新を待っています」と出す。一覧が追いつけば消える
- 比べるのは「実行中・順番待ち・待ち・完了」の4つの状態だけで、理由は比べない
- APIの形は変えない

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- `inbox-api`：1件の詳細を読んだときに、一覧との食い違いで一覧を読み直す要件を足す
- `web`：1件の画面で、一覧と状態が違うあいだの知らせの要件を足す

## Impact

- `internal/inboxserve/serve.go`（`Server.issue`）
- `web/ui/src/components/IssuePane.tsx`と、状態を比べる関数を置く`web/ui/src/lib/`
- `DESIGN.md`の1件の見出しの項
