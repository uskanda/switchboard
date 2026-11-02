# Tasks

## 1. 文書

- [x] 1.1 `docs/DECISIONS.md` にADR-042を足す
- [x] 1.2 `docs/SPEC.md` の状態図と受信箱の説明に、doneからの再開と完了の節を足す
- [x] 1.3 README.mdの実行の終わり、受信箱、WUIの説明を直す

## 2. フォージ（`internal/forge`）

- [x] 2.1 開いたPR/MRを探す操作を、GitHubとGitLabで使えるようにする（design D2）。mainとのマージで、`reuse-open-change-request` の `OpenChangeRequest` に置き換えた
- [x] 2.2 偽フォージで、開いたPRを探す操作とPRを閉じる操作を使えるようにする。mainとのマージで、`OpenChangeRequest` と `ClosePR` に置き換えた

## 3. エージェント（`internal/agent`）

- [x] 3.1 `evaluateOnce`、`decide`、`hasReply` で、doneのissueを再開の対象に入れる（design D1）。信頼済みのコメントで再開して `agent:running` になること、信頼されないコメントでは始まらないことを、失敗するテストから確かめる
- [x] 3.2 `pollReplies` と `keepNotes` の検索に `agent:done` を足す。GitLabのdoneのissueへのノートで再開することをテストで確かめる
- [x] 3.3 `afterRun` と書き戻しの再試行で、doneで終わったときも `resume` を呼ぶ
- [x] 3.4 `finish` で開いたPR/MRを使い続ける（design D2）。PRが1つのままで「Pull request updated」の投稿になること、閉じたPRのときは新しく作ることをテストで確かめる
- [x] 3.5 計画を先に立てるモードで承認済みのissueが、approvedRunとして再開することをテストで確かめる（design D5）
- [x] 3.6 許可ツールに `git merge` を足す（design D3）

## 4. プロンプト（`internal/prompt`）

- [x] 4.1 `System` と `ApprovedSystem` にbaseのブランチを渡し、衝突の解消の規則を足す。規則が入ることをテストで確かめる

## 5. 受信箱（`internal/inbox`、`inboxapi`、`internal/inboxserve`）

- [x] 5.1 状態 `done` と、項目 `DoneSince`、`ChangeURL` を足す（design D4）。完了の節の出力、通し番号、行番号のキャッシュ、JSONの項目をテストで確かめる
- [x] 5.2 `Detail` に `Done` を足し、`Reply` がdoneを受け付けてcloseを断ることをテストで確かめる
- [x] 5.3 統合テストの「doneのあとの一覧」を、完了の行が残り、closeで消える形に直す

## 6. WUI（`web/ui`）

- [x] 6.1 `api.ts` の型、一覧の完了の節、ヘッダの件数、1件の表示の返答の欄を足す。vitestで確かめる

## 7. 確かめ

- [x] 7.1 `make verify` を通す
- [ ] 7.2 テスト用のリポジトリで、done、コメント、再開、同じPRの更新、closeで一覧から消える、を一度通す
