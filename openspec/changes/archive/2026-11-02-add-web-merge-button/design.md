# Design

## Context

動機はproposal.mdの「Why」を参照。PR #73の前のmainで、この設計を縛る点は次のとおり。

- 受信箱のサービス（`switchboard inbox serve`）が持つトークンは、受信箱のトークン（`inbox_token_ref`）だけである。ADR-038、ADR-039、ADR-040は、その権限をIssuesの読み書きに限ると決めていた
- WUIには認証が無い（ADR-036、ADR-040）。返答、指示の保存、プロジェクトの追加、issueの作成のPOSTには、`sameOrigin` の検査をかけている
- エージェントのdoneの投稿のマーカーは、完了の時点のPR/MRの番号 `cr` とhead `sha` を持つ（`handle-push-and-ci-failures`）
- フォージのアダプタに、draftの解除とマージの操作は無かった
- ARCHITECTURE 4.4は「マージは人間が行う」と決めている

## Goals / Non-Goals

**Goals:**

- 画面で見たコミットだけをマージし、見ていないコミットをマージしない
- フォージに残るマージの記録を、人の身元にする

**Non-Goals:**

- 受信箱からissueを直接closeすること（Q3でCを選んだ）
- マージの方式の設定と、リモートのブランチの削除（Q2、Q4）
- CIの結果を受信箱のサービスが見ること。CIの必須化はブランチの保護に任せる（Q5）
- WUIの認証。引き続き#15の課題とする

## Decisions

### D1. 受信箱のトークンでマージする（Q1、ADR-043）

WUIから人が押すマージは、ARCHITECTURE 4.4の「マージは人間が行う」に当たる。マージは、人の身元である受信箱のトークンで行う。GitHubでは、受信箱のトークンに `Pull requests: write` と `Contents: write` を足す。GitLabでは、今の `api` スコープで足りる。権限を足さないうちは、マージだけが403で断られ、ほかの機能は変わらない。

エージェントのトークンを使う案は、botがマージした記録になるので採らなかった。3本目のトークンを設定する案は、設定が増えるわりに得るものが少ないので採らなかった。

### D2. フォージの層に `Merges` を足す

`internal/forge` に、受信箱のサービスだけが呼ぶ小さなインターフェース `Merges` を足す。メソッドは `MergeChangeRequest(ctx, owner, repo, number, sha)` の1つである。インターフェースは、`Issues` や `Repos` のように使う側ごとに分けた。

- GitHub：`GET /pulls/{n}` で状態、`draft`、`node_id`、`head.sha` を読む。headが `sha` と違えば、draftを解除する前に `ErrHeadMoved` で断る。draftなら、GraphQLの `markPullRequestReadyForReview` で解除する。続けて `PUT /pulls/{n}/merge` に `sha` を付けて送る。GraphQLは権限の不足を200の `FORBIDDEN` で返すので、`ErrForbidden` に読み替える
- GitLab：`GET merge_requests/:iid` で状態、`draft`、タイトル、`sha` を読む。draftなら、タイトルの先頭の `Draft:`、`[Draft]`、`(Draft)` を外して `PUT` する。続けて `PUT merge_requests/:iid/merge` に `sha` を付けて送る
- 応答コード：409を `ErrHeadMoved` にする。GitHubの405、422と、GitLabの405、406、422を `NotMergeableError` にし、フォージの理由を付ける。既にマージ済みか閉じたPR/MRも `NotMergeableError` にする
- 偽フォージは、draft、head、マージ済み、マージできない印をメモリで持つ

応答コードの扱いは、ROADMAPの検証項目V38とV39で実機の確かめを待つ。

### D3. 受信箱の核の `Inbox.Merge`（Q5）

一覧の要素と詳細に、最新のdoneのマーカーの `cr` と `sha` を `change_number` と `change_sha` として出す。`Inbox.Merge(ctx, m, ref, sha)` は、次の順に確かめてからマージする。

1. issueのスレッドを読み直し、openで `agent:done` であることを確かめる
2. 最新のdoneのマーカーに `cr` と `sha` があり、その `sha` がブラウザから届いた `sha` と同じことを確かめる
3. マーカーの `cr` と `sha` で `MergeChangeRequest` を呼ぶ

どれかに当たらなければ `ErrStale` で断る。ブラウザの `sha` とフォージのheadの両方を照合するので、追加のAPIの呼び出しなしで、画面で見たコミットだけをマージできる。

計画では `Inbox.Merge(ctx, ref, sha)` としていたが、実装ではマージのクライアント `m` を引数に足した。マージのクライアントを持つのは受信箱のサービスだけにし、受信箱の核で同じものを二重に持たないためである。

### D4. issueには触らない（Q3、Q4）

受信箱のサービスは、issueをcloseせず、ラベルとコメントにも触らない。closeはPR/MRの本文の `Closes #N` に任せ、worktreeの片付けはエージェントの既存の処理に任せる。baseが既定のブランチでなければissueは閉じず、受信箱に残る。そのとき同じボタンをもう一度押すと、フォージが「マージ済み」で断り、その理由がトーストに出る。PR/MRの状態を読む呼び出しは増やさない。

### D5. HTTPの二層

- 受信箱のサービス：`POST /v1/merge` を足す。`Merges` を設定したときだけ道を作る。409 `stale` は `ErrStale` と `ErrHeadMoved` に返す。409 `not_mergeable` は `NotMergeableError` に、403 `forbidden` は `ErrForbidden` に返す。マージに成功したら一覧のキャッシュを捨て、フォージを読み直させる
- `inboxapi`：`MergeRequest{Ref, SHA}`、クライアントの `Merge`、誤りの `ErrStale`、`ErrNotMergeable`、`ErrForbidden` を足す
- WUIのサーバー：`POST /api/merge` を足し、`sameOrigin` を通ったものだけを渡す（ADR-043）
- `switchboard inbox serve` の組み立て：フォージごとの受信箱のトークンのクライアントを `Merges` として渡す

### D6. WUI（Q6）

`MergeButton` を、一覧の完了の行と詳細のペインの両方に置く。一覧では、行全体が `Link` なので、ボタンをその外に置く。押すとネイティブの `<dialog>` で確認し、結果をトーストで知らせる。成功した行は、SSEの読み直しで一覧から消える。確認のダイアログの部品として `components/ui/dialog.tsx` を足し、新規issueのダイアログもこれを使うように直した。

## Risks / Trade-offs

- ［認証の無いWUIから、LANの中ならマージまでできる］→ADR-043で受け入れた。POSTには `sameOrigin` の検査をかけ、画面で見たコミットに限る
- ［受信箱のトークンの権限が広がる］→fine-grained PATを対象のリポジトリに限る。doctorが、マージに要る権限を表示する
- ［baseが既定のブランチでないと、マージしてもissueが受信箱に残る］→手でcloseする。READMEとADR-043に書いた
- ［フォージの応答コードの扱いが未確認］→V38〜V40を「未確認」としてROADMAPに記録した
