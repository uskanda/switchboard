## ADDED Requirements

### Requirement: 完了の項目のPR/MRとコミット
openで `agent:done` のissueの、一覧の要素と詳細は、PR/MRの番号を `change_number` に、コミットを `change_sha` に入れなければならない（MUST）。どちらも、エージェントの最新のdoneの投稿のマーカーから取る。
マーカーにPR/MRの番号かコミットが無いとき、APIは、その項目を空にしなければならない（MUST）。

#### Scenario: PRを作って完了したissue
- **WHEN** エージェントがPR #73を作り、マーカーに `cr=73` と `sha=17c509e` を残してdoneにしたopenのissueがある状態で、一覧を求める
- **THEN** そのissueの要素は `change_number` が73で、`change_sha` が `17c509e` である

#### Scenario: 古い完了の投稿
- **WHEN** マーカーに `cr` の無い古いdoneの投稿で完了したissueの詳細を求める
- **THEN** 詳細の `change_number` と `change_sha` は空である

### Requirement: 完了のPR/MRのマージ
APIは、issueの参照と、画面で見たコミット `sha` を受けて、そのissueのPR/MRを受信箱のトークンでマージしなければならない（MUST）。マージするのは、エージェントの最新のdoneの投稿のマーカーが名指すPR/MRで、マーカーのコミットのときに限る。
PR/MRがdraftのとき、APIは、マージの前にdraftを解除しなければならない（MUST）。GitLabでは、タイトルの先頭の `Draft:`、`[Draft]`、`(Draft)` を外して解除する。
APIは、マージの方式を指定してはならない（MUST NOT）。GitHubではAPIの既定のマージコミットになり、GitLabではプロジェクトの設定に従う。
マージに成功したとき、APIは一覧のキャッシュを捨て、フォージの読み直しを促さなければならない（MUST）。

#### Scenario: draftのPRをマージする
- **WHEN** draftのPR #73で完了したissueに、マーカーと同じ `sha` を付けてマージを求める
- **THEN** 受信箱のトークンでPR #73のdraftが解除されてからマージされる

#### Scenario: GitLabのdraftのMR
- **WHEN** タイトルが `Draft: Add a flag` のMR `!5` で完了したissueに、マージを求める
- **THEN** MRのタイトルが `Add a flag` に変わってから、MRがマージされる

### Requirement: マージの前の確認
次のどれかに当たるとき、APIはマージしてはならず、`stale` の誤りを返さなければならない（MUST NOT）。
- issueがcloseされているか、`agent:done` が付いていない
- エージェントの最新のdoneの投稿のマーカーに、PR/MRの番号かコミットが無い
- ブラウザが送った `sha` が、マーカーのコミットと違う
- PR/MRのheadが、マーカーのコミットと違う

`sha` が空のとき、APIはマージしてはならず、`invalid` の誤りを返さなければならない（MUST NOT）。
PR/MRのheadが違うとき、APIは、draftを解除してはならない（MUST NOT）。

#### Scenario: 画面を読んだ後にエージェントがpushした
- **WHEN** 画面を読んだ後に、エージェントが再開してdoneの投稿を出し直し、マーカーのコミットが変わる。その後に、古い `sha` でマージを求める
- **THEN** マージされず、`stale` の誤りが返る

#### Scenario: 人がPRにコミットを足していた
- **WHEN** doneの後に人がPRのブランチへpushし、headがマーカーのコミットと違う状態で、マージを求める
- **THEN** PRはdraftのまま残り、`stale` の誤りが返る

#### Scenario: 実行中のissue
- **WHEN** `agent:running` のissueに、マージを求める
- **THEN** マージされず、`stale` の誤りが返る

### Requirement: マージの誤り
フォージがマージを断ったとき、APIは、`not_mergeable` の誤りに、フォージが示した理由を付けて返さなければならない（MUST）。PR/MRが既にマージ済みか、閉じられているときも、`not_mergeable` を返す。
受信箱のトークンにマージの権限が無いとき、APIは、`forbidden` の誤りを返さなければならない（MUST）。誤りの文は、GitHubで要る `Pull requests` と `Contents` の書き込みの権限と、GitLabで要る `api` のスコープを名指す。

#### Scenario: CIが通っていない
- **WHEN** ブランチの保護でCIの成功を必須にしたリポジトリで、CIの失敗したPRのマージを求める
- **THEN** `not_mergeable` の誤りが、フォージの示した理由とともに返る

#### Scenario: 権限を足していないトークン
- **WHEN** Issuesの読み書きの権限だけを持つ受信箱のトークンで、GitHubのPRのマージを求める
- **THEN** `forbidden` の誤りが返る。一覧、詳細、返答は今までどおり使える

### Requirement: マージでのissueの扱い
マージのとき、APIは、issueをcloseしてはならず、issueのラベルとコメントを変えてはならない（MUST NOT）。issueのcloseは、PR/MRの本文の `Closes #N` に任せる。
マージのとき、APIは、PR/MRのブランチを消してはならない（MUST NOT）。

#### Scenario: 既定のブランチへマージする
- **WHEN** baseが既定のブランチのPRを、受信箱からマージする
- **THEN** PRの `Closes #N` でissueがcloseされ、次の一覧にそのissueは出ない

#### Scenario: 既定でないブランチへマージする
- **WHEN** baseが `develop` のPRを、受信箱からマージする
- **THEN** issueはopenのまま `agent:done` で残り、完了の項目として一覧に出る
