# Proposal

## Why

WUIでマージや返答をしても、エージェントか受信箱のサービスが次に読むまで、一覧の行は元の状態のまま残る。そのため、押した操作が無視されたように見える。依頼してから結果が届くまでの間は処理中だと示し、行が並び替わるときは動きで追えるようにする。

## What Changes

- マージ、返答、新規issueの作成が成功したとき、その行に処理中の印を付ける。印はタブのsessionStorageに持ち、再読み込みしても残す
- 印の付いた行は、Spinnerと、ゆっくり明滅する「マージ処理中」「処理中」を出す。返答と新規issueの行は実行中の節へ移し、マージの行は完了の節に残す
- 新規issueは、作成の応答と入力したタイトルから仮の行を作り、実行中の節に出す
- 一覧でそのissueの状態が変わるか、一覧から消えると、印を外す。10分で外れなければ外し、トーストで伝える
- タブのタイトルとヘッダの件数は、印を反映した節で数える
- 一覧の行の位置が変わったときは、行を元の位置から新しい位置へ動かす。新しく現れた行はフェードインさせる
- PRODUCT.mdの動きの方針に、処理中の表示と並べ替えを例外として足す

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

- `web`: 操作の後の処理中の表示と、一覧の並べ替えの動きを足す。タブのタイトルとヘッダの件数を、処理中の印を反映した件数にする

## Impact

- `web/ui/src/lib/pending.ts`、`web/ui/src/lib/flip.ts`（新規）
- `web/ui/src/lib/`の`inbox.tsx`と`title.ts`、`web/ui/src/index.css`
- `web/ui/src/components/`の`InboxList.tsx`、`Header.tsx`、`MergeButton.tsx`、`IssuePane.tsx`、`NewIssue.tsx`
- `PRODUCT.md`
- 受信箱のサービス、`inboxapi`の型、依存のパッケージは変えない
