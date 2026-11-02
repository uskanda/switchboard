# Tasks

## 1. 文書

- [x] 1.1 `docs/DECISIONS.md` にADR-043を足す（design D1、D4）
- [x] 1.2 README.mdのトークンの表で、受信箱のトークンの権限の説明を直す。同じ説明を直す先は、`config/switchboard.example.yaml`、`docs/SECURITY.md` のT5と4節、`docs/DESIGN.md` である
- [x] 1.3 `docs/ARCHITECTURE.md` の4.4に、WUIからのマージを書き足す
- [x] 1.4 DESIGN.mdに、マージのボタンと確認のダイアログの見た目を書く
- [x] 1.5 `docs/ROADMAP.md` の検証項目にV38〜V40を「未確認」として足す

## 2. フォージ（`internal/forge`）

- [x] 2.1 `forge` に、インターフェース `Merges`、誤り `ErrHeadMoved`、`NotMergeableError` を足す（design D2）
- [x] 2.2 GitHubの `MergeChangeRequest` を作る。draftの解除からマージまでの呼ぶ順、`sha` の受け渡し、headが違うときにdraftを解除しないことをhttptestのテストで確かめる。応答コードと誤りの対応、GraphQLの `FORBIDDEN` も同じテストで確かめる
- [x] 2.3 GitLabの `MergeChangeRequest` を作る。タイトルからdraftの印を外すこと、`sha` の受け渡し、応答コードと誤りの対応をhttptestのテストで確かめる
- [x] 2.4 `fake` の偽フォージにマージの状態を持たせ、テストで確かめる

## 3. 受信箱の核（`internal/inbox`）

- [x] 3.1 `Item` と `Detail` に `ChangeNumber` と `ChangeSHA` を足す（design D3）
- [x] 3.2 `Inbox.Merge` を作る。状態の違い、`sha` の不一致、`cr` の無いマーカー、成功をテストで確かめる

## 4. HTTP（`inboxapi`、`internal/inboxserve`、`web/internal/server`、`cmd/switchboard`）

- [x] 4.1 `inboxapi` に `MergeRequest`、クライアントの `Merge`、誤りのコード `stale`、`not_mergeable`、`forbidden` を足す。クライアントのテストで確かめる
- [x] 4.2 `internal/inboxserve` に `POST /v1/merge` を足す（design D5）。道の有無、成功のときのマージ、誤りと応答コードの対応をテストで確かめる。成功のときは一覧のキャッシュを捨てる
- [x] 4.3 `web/internal/server` に `POST /api/merge` を足す。`sameOrigin` の検査と誤りの対応をテストで確かめる
- [x] 4.4 `switchboard inbox serve` の組み立てで、受信箱のトークンのクライアントを `Merges` として渡す。`cmd/switchboard` のテストで確かめる
- [x] 4.5 doctorで、受信箱のトークンについて、マージに要る権限を表示する。doctorのテストで確かめる

## 5. WUI（`web/ui`）

- [x] 5.1 `lib/api.ts` に `merge` と、`change_number`、`change_sha` の型を足す
- [x] 5.2 `components/ui/dialog.tsx` を足し、新規issueのダイアログもこれを使うように直す
- [x] 5.3 `MergeButton` を作り、一覧の完了の行と詳細のペインに置く（design D6）。ボタンの有無、「やめる」で送らないこと、成功のトースト、409と403の誤りのトーストを、vitestの9件で確かめる

## 6. 確かめ

- [x] 6.1 `make verify` を通す。PR #73のCIの `verify` が通った
- [ ] 6.2 GitHubのfine-grained PATで、draftの解除、`sha` を付けたマージ、権限の無いときの403、409と405の応答を確かめる（V38）
- [ ] 6.3 GitLabで、タイトルからのdraftの解除、`sha` を付けたマージ、断られたときの応答コードを確かめる（V39）
- [ ] 6.4 usknasの常駐のサービスのWUIから、sandboxのdraftのPRをマージし、`Closes` でissueが閉じて受信箱から消えることを確かめる（V40）
