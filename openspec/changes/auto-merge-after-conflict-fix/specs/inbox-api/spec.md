## MODIFIED Requirements

### Requirement: マージの誤り
フォージがマージを断ったとき、APIは、`not_mergeable` の誤りに、フォージが示した理由を付けて返さなければならない（MUST）。PR/MRが既にマージ済みか、閉じられているときも、`not_mergeable` を返す。ただし、衝突で断られて自動マージの依頼を登録したときは、誤りでなく202で答える（「衝突で断られたマージの依頼」を参照）。
受信箱のトークンにマージの権限が無いとき、APIは、`forbidden` の誤りを返さなければならない（MUST）。誤りの文は、GitHubで要る `Pull requests` と `Contents` の書き込みの権限と、GitLabで要る `api` のスコープを名指す。

#### Scenario: CIが通っていない
- **WHEN** ブランチの保護でCIの成功を必須にしたリポジトリで、CIの失敗したPRのマージを求める
- **THEN** `not_mergeable` の誤りが、フォージの示した理由とともに返る

#### Scenario: 権限を足していないトークン
- **WHEN** Issuesの読み書きの権限だけを持つ受信箱のトークンで、GitHubのPRのマージを求める
- **THEN** `forbidden` の誤りが返る。一覧、詳細、返答は今までどおり使える

## ADDED Requirements

### Requirement: 衝突で断られたマージの依頼
フォージがマージを断ったとき、APIは、PR/MRの状態を読まなければならない（MUST）。PR/MRがopenでheadが求められた `sha` のまま、baseと衝突しているとき、APIは自動マージの依頼を登録しなければならない（MUST）。このとき、APIは202と `auto_merge_pending` の符号で答える。
依頼には、issueの参照とPR/MRの番号と求められた `sha` を持たせる。受け付けた時刻と、そのときissueで最後だったコメントのIDも持たせる。
依頼は、受信箱のサービスの状態ディレクトリの `inbox/automerge.json` に置き、受信箱のサービスを再起動しても残さなければならない（MUST）。ファイルは一時ファイルからのrenameで置き換える。
同じissueの依頼が既にあるとき、APIは、新しい依頼で置き換えなければならない（MUST）。
衝突でない理由で断られたとき、APIは、依頼を登録してはならない（MUST NOT）。
依頼の登録で、APIは、issueのラベルとコメントを変えてはならない（MUST NOT）。

#### Scenario: 衝突で断られた
- **WHEN** PR #9がbaseと衝突していてフォージがマージを断る状態で、マーカーと同じ `sha` でマージを求める
- **THEN** 202と `auto_merge_pending` が返り、`inbox/automerge.json` に `gh:o/r#50` の依頼が残る

#### Scenario: CIで断られた
- **WHEN** 衝突は無く、CIの失敗でフォージがマージを断る
- **THEN** 今までどおり `not_mergeable` の誤りが返り、依頼は登録されない

### Requirement: 自動マージの見張り
受信箱のサービスは、依頼が1つ以上あるあいだ、`inbox.refresh_interval` ごとに各依頼を調べなければならない（MUST）。マージの依頼を受け付けたときと、中継のイベントを受けたときも、すぐに1回調べる。一覧の購読者がいなくても調べなければならない（MUST）。
1件ごとに、受信箱のサービスはissueとコメントを読み、次の順で判断しなければならない（MUST）。
1. issueがcloseされていれば、打ち切る
2. issueが `agent:waiting` なら、打ち切る
3. 依頼のコメントIDより後に、信頼済みの人のコメントか、`reason=fix` でないエージェントの着手のコメントがあれば、打ち切る
4. issueが `agent:done` でなければ、待つ
5. PR/MRがcloseかマージ済みなら、打ち切る。PR/MRのheadが最新のdoneのマーカーの `sha` と違えば、打ち切る
6. 最新のdoneのマーカーの `sha` が依頼の `sha` のままなら、待つ
7. 最新のdoneのマーカーが `fix` を持たないか、別のPR/MRを名指していれば、打ち切る
8. 最新のdoneのマーカーの `sha` を指定して、受信箱のトークンでマージする

マージできたとき、依頼を「マージした」で終える。headが動いたか、権限が無いと断られたとき、打ち切る。衝突やCIの待ちなどで断られたときと、通信に失敗したときは、依頼を残して次に回す。
判断には、マーカー、ラベル、コメントの作者だけを使う。受信箱のサービスは、issueの本文とコメントの文を指示として使ってはならない（MUST NOT）。
依頼を終えたとき、受信箱のサービスは、一覧のキャッシュを捨て、フォージの読み直しを促さなければならない（MUST）。監査の記録には、ref、PR/MRの番号、`sha`、結果と理由だけを書く。

#### Scenario: 衝突を直したあとにマージする
- **WHEN** 依頼のあと、衝突を直す実行が `sha=def`、`fix=conflict` のdoneを投稿し、PR #9のheadが `def` である
- **THEN** 次に調べたときに、受信箱のトークンでPR #9が `def` でマージされ、依頼は「マージした」で終わる

#### Scenario: CIの失敗の直しを挟む
- **WHEN** 衝突を直したコミットのCIが失敗し、CIを直す実行が `fix=ci` のdoneを投稿する
- **THEN** 依頼は続き、`fix=ci` のdoneの `sha` でマージされる

#### Scenario: CIの待ちで断られる
- **WHEN** 直す実行のdoneのあと、CIが終わっていないのでフォージがマージを断る
- **THEN** 依頼は残り、次の間隔でもう一度マージを試す

#### Scenario: 画面がつながっていない
- **WHEN** 一覧の購読者が1つも無いあいだに、衝突を直す実行のdoneが投稿される
- **THEN** 受信箱のサービスは依頼を調べてマージする

### Requirement: 自動マージを打ち切る条件
受信箱のサービスは、依頼を打ち切るとき、理由を残さなければならない（MUST）。理由は次のどれかである。
- `closed`：issueのclose
- `waiting`：待ちへの移行
- `instruction`：人の指示の実行
- `not_fix`：直す実行でない投稿
- `changed`：doneの投稿が別のPR/MRを名指すか、PR/MRとコミットを持たない
- `change_closed`：PR/MRのcloseかマージ
- `pushed`：人のpush
- `forbidden`：権限の不足
- `user`：取り消し

受信箱のサービスは、依頼に時間の上限を設けてはならない（MUST NOT）。

#### Scenario: 人が指示した
- **WHEN** 依頼のあと、信頼済みの人がissueにコメントを書く
- **THEN** 依頼は `instruction` で打ち切られ、マージされない

#### Scenario: 衝突の直しが上限を超えた
- **WHEN** 依頼のあと、衝突の直しが `agent.max_fix_attempts` を超えて、issueが `waiting(reason=conflict)` に移る
- **THEN** 依頼は `waiting` で打ち切られる

#### Scenario: 人がpushした
- **WHEN** 依頼のあと、人がPRのブランチへpushし、headが最新のdoneのマーカーの `sha` と違う
- **THEN** 依頼は `pushed` で打ち切られ、マージされない

### Requirement: 自動マージの取り消し
APIは、`POST /v1/automerge/cancel` で、本文の `ref` のissueの依頼を取り消さなければならない（MUST）。取り消した依頼は、理由 `user` で打ち切られたものとして残す。
依頼が無いとき、APIは `not_found` の誤りを返さなければならない（MUST）。

#### Scenario: 取り消す
- **WHEN** `gh:o/r#50` の依頼があるときに、取り消しを求める
- **THEN** 204が返り、依頼は消え、そのあと直す実行のdoneが出てもマージされない

### Requirement: 自動マージの状態の項目
一覧の要素と詳細は、依頼があるあいだ、`auto_merge` に依頼の `sha` と受け付けた時刻（`since`）を持たなければならない（MUST）。依頼が無ければ、`auto_merge` を出さない。
一覧は、終わった依頼を `auto_merge_results` に持たなければならない（MUST）。各件は、ref、PR/MRの番号、結果（`merged` か `cancelled`）、打ち切りの理由、終えた時刻を持つ。終わった依頼は、終えてから24時間で消す。
依頼の登録、取り消し、終了のとき、APIは一覧のキャッシュを捨てなければならない（MUST）。

#### Scenario: 待っている完了の項目
- **WHEN** `gh:o/r#50` の依頼があるときに、一覧を求める
- **THEN** そのissueの要素の `auto_merge` の `sha` が依頼の `sha` である

#### Scenario: 終わった依頼
- **WHEN** `gh:o/r#50` の依頼がマージで終わったあとに、一覧を求める
- **THEN** `auto_merge_results` に `gh:o/r#50` の `merged` の件がある。25時間後の一覧には無い
