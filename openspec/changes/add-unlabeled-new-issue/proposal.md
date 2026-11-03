# Proposal

## Why

「後で作りたい」ものが出てきたとき、受信箱のWeb表示（WUI）から気軽にissueを入れておきたい（issue #107）。今の新規issueのダイアログは開始ラベルを必ず1つ付けるので、作ったissueにはエージェントがすぐ着手してしまう。

## What Changes

- 受信箱のAPIの開始ラベルの種別に `none`（ラベルなし）を足す。`none` はラベル名を持たず、設定に関係なく、作れる先の一覧の最後に常に出る。
- 種別 `none` でissueを作ると、ラベルを1つも付けずに作る。空の種別や未知の種別は今までどおり `invalid` で拒む。
- WUIの新規issueのダイアログで、開始ラベルの最後に「ラベルなし（実装開始しない）」を出す。この選択肢にはラベル名を出さない。初期値は `ready` のまま。
- ADR-048を足し、ADR-040の「開始ラベルを1つ付ける」を「1つ付けるか、付けない」に改める。エージェントはラベルなしのissueには着手せず、あとで本人が開始ラベルを付けたときに着手する。

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

- `inbox-api`：「issueを作れる先の一覧」で種別 `none` を返す。「issueの作成」で、`none` ならラベルを付けずに作る。
- `web`：「新規issueのダイアログ」で、開始ラベルの最後に「ラベルなし（実装開始しない）」を出す。

## Impact

- `inboxapi/inboxapi.go`（種別の定数）、`internal/inboxserve/issues.go`（一覧と作成）
- `web/ui/src/lib/api.ts`、`web/ui/src/components/NewIssue.tsx`
- `docs/DECISIONS.md`（ADR-048）、`docs/SPEC.md`、`README.md`、`DESIGN.md`
- フォージの層は変えない。GitHubとGitLabは、ラベルの空の一覧でissueを作れる
