# Proposal

## Why

Phase 5で作ったWUIは、一覧の画面と1件の画面が分かれている。返答のたびに一覧と1件を行き来する必要がある。また、画面は開いたときの内容のままである。待ちが増えても、返答したissueが待ちでなくなっても、再読み込みするまで変わらない。issue #19は、ヘッダと2つのペインの画面にすることと、表示中の更新に追随することを求めている。

## What Changes

- WUIの画面を、ヘッダと2つのペインで組み直す。左に待ちの一覧、右に選んだissueの全文と返答の欄を出す。狭い幅では、ペインを1つずつ出す
- `switchboard-web` に `/api/events` を足す。開いている画面へ、一覧をServer-Sent Eventsで送る。一覧は5秒ごとに受信箱のサービスへ求め直し、変わったときだけ送る。返答のあとは、すぐに求め直す
- 画面は、選んだissueの一覧の項目が変わったか、一覧から消えたときに、詳細を取り直す
- ヘッダに、自動更新がつながっているかを出す
- `switchboard inbox serve` の一覧のキャッシュを、60秒から30秒に縮める
- ADR-038を足し、DESIGN.mdの組み方とREADME.mdを更新する

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

- `web`：画面の組み方と、自動更新
- `inbox-api`：一覧のキャッシュの長さ

## Impact

- `web/internal/server`、`web/cmd/switchboard-web`、`web/ui/src`
- `internal/inboxserve`
- DESIGN.md、README.md、docs/DECISIONS.md
