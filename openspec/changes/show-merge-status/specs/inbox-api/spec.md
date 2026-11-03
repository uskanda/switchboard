## ADDED Requirements

### Requirement: マージの前の状態の読み出し
issueの参照と画面で見たコミット `sha` を受けたとき、APIは、そのissueのPR/MRの状態を受信箱のトークンで読んで返さなければならない（MUST）。読むのは、エージェントの最新のdoneの投稿のマーカーが名指すPR/MRと、マーカーのコミットのCIである。
応答には次の項目を入れる。
- `state`：PR/MRの状態（`open`、`closed`、`merged`）
- `head_moved`：PR/MRのheadがマーカーのコミットと違うか
- `mergeable`：衝突の有無
- `ci`：CIの読み取りの結果（`ok`、`none`、`forbidden`）
- `checks`：チェックの一覧

`mergeable` は `clean`、`conflict`、`computing` のどれかとし、フォージが計算中のときは `computing` にする。チェックの各要素には、名前、`success`、`failure`、`running`、`other` のどれかの結果、フォージのURLを入れる。
APIは、PR/MRとissueに何も書いてはならない（MUST NOT）。

#### Scenario: CIが失敗しているPR
- **WHEN** draftのPR #73で完了したissueのコミットに、成功のチェックが1つ、失敗のチェックが1つある状態で、マーカーと同じ `sha` を付けて状態を求める
- **THEN** `ci` が `ok` で、`checks` に2つのチェックが結果とURLとともに入って返る。PRはdraftのまま残る

#### Scenario: 衝突のあるPR
- **WHEN** baseと衝突するPRで完了したissueの状態を求める
- **THEN** `mergeable` が `conflict` で返る

#### Scenario: フォージが計算中
- **WHEN** GitHubの `mergeable` が `null` のPRで完了したissueの状態を求める
- **THEN** `mergeable` が `computing` で返る

#### Scenario: 人がPRにコミットを足していた
- **WHEN** doneの後に人がPRのブランチへpushし、headがマーカーのコミットと違う状態で、状態を求める
- **THEN** `head_moved` がtrueで返る

#### Scenario: チェックの無いコミット
- **WHEN** CIの無いリポジトリのPRで完了したissueの状態を求める
- **THEN** `ci` が `none` で、`checks` が空で返る

### Requirement: CIを読む権限の無い受信箱のトークン
受信箱のトークンにCIを読む権限が無いとき、APIは、誤りを返さずに、`ci` を `forbidden` にした状態を返さなければならない（MUST）。そのとき `checks` は空にし、PR/MRの状態と衝突の有無は返す。

#### Scenario: 権限を足していないGitHubのトークン
- **WHEN** Actionsの読み取りの権限を持たない受信箱のトークンで、衝突のあるPRの状態を求める
- **THEN** `ci` が `forbidden`、`mergeable` が `conflict` で返る

### Requirement: マージの前の状態の読み出しの前提
次のどれかに当たるとき、APIは、状態を読んではならず、`stale` の誤りを返さなければならない（MUST NOT）。
- issueがcloseされているか、`agent:done` が付いていない
- エージェントの最新のdoneの投稿のマーカーに、PR/MRの番号かコミットが無い
- ブラウザが送った `sha` が、マーカーのコミットと違う

`sha` が空のとき、APIは、状態を読んではならず、`invalid` の誤りを返さなければならない（MUST NOT）。

#### Scenario: 実行中のissue
- **WHEN** `agent:running` のissueの状態を求める
- **THEN** `stale` の誤りが返る

#### Scenario: 画面を読んだ後にエージェントがpushした
- **WHEN** 画面を読んだ後に、エージェントがdoneの投稿を出し直し、マーカーのコミットが変わる。その後に、古い `sha` で状態を求める
- **THEN** `stale` の誤りが返る
