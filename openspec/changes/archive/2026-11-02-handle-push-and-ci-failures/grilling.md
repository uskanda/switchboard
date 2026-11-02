# grilling 記録: handle-push-and-ci-failures

pushの失敗と、エージェントが作った変更要求（PR/MR）のCIの失敗を、エージェントが読み取って対処できるようにする計画である。今のエージェントはCIを見ていない。pushが失敗したときも、`pushFailed` のコメントを書いて `waiting(reason=error)` にするだけである。先行する変更 `reuse-open-change-request`（開いた変更要求の再利用と、GitHubのエラーの詳細の表示）の後に入れる。

## 決定

| 決定 | 選択 | 出典 |
|---|---|---|
| 範囲 | pushの失敗とCIの失敗を1つの変更で扱う。どちらも、失敗の内容をClaudeに渡して直させる同じ仕組みを使い、回数の上限とコメントの形を共有する | 第1ラウンド Q1 |
| CIを待つ時機 | 変更要求を作った時点で、今までどおり `agent:done` にする。そのうえでエージェントが自分のpushしたコミットのCIを見張り、失敗したら直す実行を始めてissueを `agent:running` に戻す。CIを待つ間は、実行の枠も状態ラベルもふさがない | 第1ラウンド Q2 |
| CIの結果の知り方 | GitHubもGitLabもポーリングで知る。中継は変えない。GitHubのwebhookは中継に届かない（ADR-007）ので、GitHubではポーリングが要る。GitLabのPipeline Hookは、遅さが問題になってから足す | 第1ラウンド Q3 |
| pushの失敗の分け方 | stderrで4つに分ける。(1)一時的な失敗（ネットワーク、5xx、タイムアウト）：同じ実行の中で間隔を空けて数回やり直し、それでも駄目なら `waiting(reason=error)`。(2)non-fast-forward：fetchしてClaudeに直させる。強制pushはしない。(3)権限・認証：`waiting(reason=push)`。トークンの権限を確かめるよう案内する。(4)リモートの規則による拒否（ブランチの保護、pre-receive hook、push rule、サイズの上限）：`waiting(reason=push)` にし、リモートのメッセージを書く。直させたければ人が返信する。自動で直させるのは(2)とCIの失敗だけ | 第1ラウンド Q4 |
| GitHubのトークンの権限 | エージェントのfine-grained PATに `Actions: Read` と `Commit statuses: Read` を足す。`switchboard doctor` で不足を知らせる。Checks APIをfine-grained PATで読めるかは、ROADMAPの検証項目で確かめてから使う。GitLabは今の `api` スコープで足りる | 第1ラウンド Q5 |
| CIのログの扱い | 非信頼のデータとして、nonce付きの区切りでプロンプトに入れる。中の文は指示として扱わない | 第1ラウンド Q6 |
| 見張る対象と期間 | エージェントが自分でpushしたhead SHAだけを見張る。完了のコメントのマーカーに `sha` と `cr`（変更要求の番号）を足して記録し、再起動しても担当の端末が続きから見張れるようにする。見張りの間は `poll_interval` ごとに問い合わせる。CIが全部終わったとき、変更要求が閉じられたかマージされたとき、ブランチのheadが別のSHAに変わったとき、pushから6時間たったときにやめる。pushから10分たってもCIが1つも現れなければ、CIの無いリポジトリとみなしてやめる。時間切れは監査ログにだけ残し、コメントは書かない | 第2ラウンド Q7 |
| 失敗の判定 | そのSHAのcheck run、commit status、GitLabのpipelineが全部終わってから判定する。`failure` と `timed_out` を失敗とし、`cancelled`、`skipped`、`neutral` は数えない。必須のチェックかどうかは区別しない | 第2ラウンド Q8 |
| 自動で直す回数の上限 | CIの失敗とnon-fast-forwardを合わせて、1つのissueで数える。既定は2回で、設定は `<agent.max_fix_attempts>`（0は自動で直さない）。回数はマーカー（`reason=fix`）から数え、信頼済みの人のコメントか始めるラベルで0に戻す。上限に達したら `waiting(reason=ci)` にする | 第2ラウンド Q9 |
| ログをどこまで渡すか | GitHub Actionsでは、失敗したjobを最大3つまで、jobごとにログの末尾200行（20KB）を渡す。ANSIの色の符号と行頭の時刻を消し、秘密の形式の文字列は伏せる。チェック名、失敗したstep、URLも渡す。commit statusだけの外部CIは、名前、説明、URLだけを渡す。GitLabは失敗したjobのtraceを同じ上限で渡す。渡す先はプロンプトだけ | 第2ラウンド Q10 |
| 直す実行の流れ | `agent:done` から `agent:running` に戻し、失敗したチェックと回数（例「CIが失敗したので直します（1/2）」）を書いた着手のコメントを付ける。新しいセッションで、issue、コメント、失敗の情報を渡し、CIの失敗を直す作業を指示する。その後は、完了の検証、出力検査、push（既存の変更要求を再利用）、`agent:done`、再び見張る、と進む。保護パスを変えないと直せないときは、今までどおり `waiting(reason=verify)` になる。計画のモードのissueでも、承認済みの実装として直し、計画には戻さない | 第2ラウンド Q11 |
| non-fast-forwardを直す実行 | エージェントが `origin/switchboard/issue-<n>` をfetchし、pushのエラーをプロンプトに入れて実行する。Claudeに許すのは `git merge` だけで、`git rebase` と強制pushは許さない。人がpushした履歴を書き換えないため | 第2ラウンド Q12 |
| 上限に達したときのコメント | `waiting(reason=ci)` には、失敗したチェックの名前と結論とURL、試した回数と各回の要約、ログの末尾40行（`<details>` の中）を書く。`waiting(reason=push)` には、分類、トークンを伏せたgitのstderr、人がすべきことの案内を書く | 第2ラウンド Q13 |
| 受信箱での見え方 | `waiting(reason=ci)` と `waiting(reason=push)` は、ほかの待ちと同じく受信箱に出し、reasonを表示用の言葉にする。CIを見張っている `agent:done` のissueは受信箱に出さない | 第3ラウンド Q14 |
| ADR | ADR-041を足す。人のきっかけが無くても自分のPRのCIの結果で実行を始めること、GitHubのトークンに2つの読み取りの権限を足すことを残す。SECURITY.mdの4章も書き換える | 第3ラウンド Q15 |
| 並列数と順番待ち | 直す実行も、全体とプロジェクト単位の並列数の上限、順番待ち、端末をまたいだ重複の防止に従う | 第3ラウンド Q16 |
| Checks APIを読めないとき | 検証項目でfine-grained PATがcheck runを読めないと分かったら、GitHub Actionsのworkflow runとcommit statusだけで判定する | 第3ラウンド Q17 |

## 後回しにしたもの

- GitLabのPipeline Hookを中継のイベントの種別に足すこと（CIの結果をポーリングより早く知る）。遅さが問題になってから別の変更で扱う
- リモートの規則による拒否のうち、コミットメッセージの形式などClaudeでも直せるものを、自動で直させること

## 状態

frontierは空。共有理解は2026-11-02に確認済み。
