# Tasks

前提：`reuse-open-change-request` がmainに入っていること。

## 1. 検証項目

- [ ] 1.1 `docs/ROADMAP.md` の検証項目にV35を足す。V35は、fine-grained PATでcheck runを読めるかを確かめる。sandboxのリポジトリで、`Checks` の権限を付けたトークンと付けないトークンを使う。それぞれで `/commits/<sha>/check-runs` の応答を記録する。結果の行が表に入っていることで確かめる
- [ ] 1.2 `Actions: Read` と `Commit statuses: Read` を足したトークンを使う。workflow run、job、jobのログ、combined statusを読めることを確かめる。結果はV36としてROADMAP.mdに記録する。結果の行で確かめる

## 2. フォージのCIの読み取り

- [x] 2.1 `internal/forge` に `<forge.CI>`（`CommitChecks`、`CheckLog`、`ChangeRequestHead`）とチェックの型を足し、fakeを実装する。fakeのテストで、終わっていないチェック、結論、ログを返せることを確かめる
- [x] 2.2 `internal/forge/github` に実装を書く。Actionsのworkflow runとjob、jobのログ、combined statusを使う。check runは、1.1の結果が「読める」のときだけ足す。`httptest` のテストで、`head_sha` での絞り込みとログの取得を確かめる。403が `forge.ErrForbidden` になることも確かめる
- [x] 2.3 `internal/forge/gitlab` に実装を書く。pipeline、job、trace、コミットのstatusを使う。偽のGitLabのテストで、同じ3つを確かめる
- [x] 2.4 `<forge.CI>` の利用側に、ログの整形を書く。jobは最大3つで、末尾は200行と20KBのうち短いほうまでとする。ANSIの色の符号と行頭の時刻を取り除き、秘密の形式の文字列を伏せ字にする。単体テストで、5,000行のログと `ghp_` を含むログを確かめる

## 3. マーカーと設定

- [x] 3.1 `internal/marker` に `sha`、`cr` を足し、`reason` の値に `fix`、`ci`、`push` を足す。往復のテストと、前の版のマーカーを読めるテストで確かめる
- [x] 3.2 `internal/config` に `<agent.max_fix_attempts>`（既定2、負は誤り）を足す。spec `config` の2つのシナリオのテストで確かめる。READMEの設定の例を書き足す

## 4. pushの失敗の分類

- [x] 4.1 `internal/workspace` の `Push` が、失敗を `transient`、`nonFastForward`、`auth`、`rejected` に分けた誤りを返すようにする。GitHubとGitLabの実際の出力の例をテストに置いて確かめる。例は、`fetch first`、`GH006`、`pre-receive hook declined`、`Authentication failed`、`Could not resolve host` である
- [x] 4.2 `internal/workspace` に、リモートの `switchboard/issue-<n>` をfetchする操作を足す。ローカルのbareリポジトリを使うテストで、fetchの後に `origin/switchboard/issue-<n>` があることを確かめる
- [x] 4.3 `internal/workspace` に、pushの出力の伏せ字を書く。トークンを含むURLの出力で、伏せ字になることをテストで確かめる

## 5. 完了の処理でのpushの失敗

- [x] 5.1 `finish` で、一時的な失敗を5秒と20秒の間隔で3回まで試みる。偽のpushで、2回目に成功すると `agent:done` になり、3回とも失敗すると `waiting(reason=error)` になることをテストで確かめる
- [x] 5.2 `finish` で、権限・認証と規則による拒否を `waiting(reason=push)` にする。コメントには、分類、伏せ字の出力、人がすべきことを書く。英語と日本語の定型文を `messages.go` に足す。2つの分類のテストで確かめる
- [x] 5.3 `finish` で、non-fast-forwardのときはfetchしてから `reason=fix` の着手のコメントを付ける。issueは `agent:running` のまま `fixRun` を始める。上限に達していれば `waiting(reason=push)` にする。強制pushをしないことも含めてテストで確かめる
- [x] 5.4 `outcome` で、完了のコメントのマーカーに `sha` と `cr` を書く。spec `run-outcome` の「完了のマーカー」のテストで確かめる

## 6. 直す実行

- [x] 6.1 `<plan.kind>` に `fixRun` を足し、失敗の情報を持たせる。直した回数をマーカーから数える関数を書く。リセットの点（信頼済みのコメント、`mode` を持つ着手のコメント）の前後で数が変わることをテストで確かめる
- [x] 6.2 `internal/prompt` に、失敗の情報を入れるnonce付きの非信頼の区切りと、`fixRun` の追記の指示を足す。ログの中の指示の形の行が区切りの中にだけ現れることをテストで確かめる
- [x] 6.3 `settings` で、non-fast-forwardの `fixRun` だけに `Bash(git merge *)` を足し、`git rebase` をどの実行でもdenyに入れる。設定の中身のテストで確かめる
- [x] 6.4 `fixRun` の終わりを、検証、出力検査、push、既存の変更要求の再利用、完了のコメントの順に通す。ワークフローを変えた直しが `waiting(reason=verify)` になることもテストで確かめる
- [x] 6.5 `fixRun` が、計画のモードのissueでも計画の判定を通らずに実装として動くことをテストで確かめる

## 7. CIの見張り

- [x] 7.1 `internal/agent` で、見張りの一覧をメモリに持ち、完了の書き戻しで足す。起動と照合のときに `agent:done` のissueを検索し、6時間以内の `sha` を持つ完了のマーカーから作り直す。再起動のシナリオと、`sha` の無いマーカーのシナリオのテストで確かめる
- [x] 7.2 `internal/agent` で、ポーリングのたびに一覧の各issueのCIの状態と変更要求のheadを問い合わせる。中継につながっている間も `poll_interval` で回す。中継のモードのテストで確かめる
- [x] 7.3 `internal/agent` に、終わりの条件を実装する。条件は4つある。CIが全部終わったこと、変更要求が閉じたかマージされたこと、headが変わったこと、issueが `agent:done` でないことである。加えて、10分たってもCIが無いときと、6時間の時間切れ（監査ログだけ）も扱う。spec `ci-watch` のシナリオごとのテストで確かめる
- [x] 7.4 `internal/agent` に、失敗の判定（全部終わってから、`failure` と `timed_out` だけ）を実装する。一部が動いているシナリオと、取り消されたjobのシナリオのテストで確かめる
- [x] 7.5 `internal/agent` で、CIが失敗して上限未満なら、空いた枠で `agent:running` に変える。着手のコメント（`reason=fix`、「（k/N）」）を付け、`fixRun` を始める。枠が無ければ次のポーリングでやり直す。並列数の上限と、端末をまたいだ重複の防止のテストで確かめる
- [x] 7.6 `internal/agent` で、上限まで直していれば `waiting(reason=ci)` にする。コメントには、失敗したチェック、各回の要約、`<details>` に入れたログの末尾40行を書く。`<agent.max_fix_attempts>` が0の設定と2の設定のテストで確かめる
- [x] 7.7 `internal/agent` で、403のときは見張りをやめて監査ログに `<ci.forbidden>` を残し、プロジェクトごとに1回だけログに出す。テストで確かめる

## 8. 周辺

- [x] 8.1 `switchboard doctor` で、GitHubのプロジェクトのActionsとcombined statusを読んでみて、403なら警告にする。偽のGitHubのテストで確かめる
- [x] 8.2 `web/ui` の一覧と1件の画面で、理由 `push` と `ci` を表示し、1件の画面に案内を出す。`web/ui` のテストで、spec `web` の2つのシナリオを確かめる。DESIGN.mdとPRODUCT.mdに理由の表示の記述があれば合わせる
- [x] 8.3 `internal/notify` の待ちの通知で、理由 `push` と `ci` を送る。spec `notify` の「CIの直しが上限に達した」のテストで確かめる

## 9. 文書

- [x] 9.1 `docs/DECISIONS.md` にADR-041を足す。人のきっかけが無くても自分のPRのCIの結果で実行を始めること、GitHubのトークンに2つの読み取りの権限を足すことを書く。grillingの第1ラウンドQ2、Q5、第3ラウンドQ15を出典にする
- [x] 9.2 `docs/SECURITY.md` の4章に、GitHubの `Actions: Read` と `Commit statuses: Read` を足す。CIのログを非信頼のデータとして扱うことを、3章のT1かT9に書き足す。READMEのトークンの作り方も合わせる
- [x] 9.3 `docs/SPEC.md` と `docs/ARCHITECTURE.md` に、CIの見張りと直す実行の流れを書き足す

## 10. 結合の確認

- [ ] 10.1 usknasの常駐のサービスで、sandboxのリポジトリに、わざと失敗するテストを含むissueを立てる。PRができた後にCIが失敗し、直す実行が同じPRにpushすることを確かめる。CIが通り、`agent:done` のまま見張りが終わるまでを見る。結果をROADMAP.mdの検証項目に記録する
- [ ] 10.2 sandboxで、人が `switchboard/issue-<n>` にコミットを足してから返信して再実行させる。non-fast-forwardから `git merge` で直してpushされることを確かめ、記録する
- [ ] 10.3 `make verify` が通ることを確かめる
