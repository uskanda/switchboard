## MODIFIED Requirements

### Requirement: 返答
APIは、指定したissueに返答を投稿しなければならない（MUST）。投稿の前の確認は、`switchboard reply` と同じにする。issueを取り直して、openで `agent:waiting` か `agent:done` が付いていることを確かめ、空の返答とマーカーで終わる返答は拒む。
どちらでもないときは、投稿せずに、待ちでないことを区別できる誤りを返さなければならない（MUST）。

#### Scenario: 既に再開していた
- **WHEN** `agent:waiting` が外れて `agent:running` が付いたissueへ、返答を送る
- **THEN** 投稿されず、待ちでないことを示す誤りが返る

#### Scenario: 完了のissueに返答する
- **WHEN** openのまま `agent:done` のissueに返答を送る
- **THEN** 返答が投稿される

## ADDED Requirements

### Requirement: 完了の項目
一覧の要素は、openのまま `agent:done` のissueについて、状態 `done` と行番号を持たなければならない（MUST）。完了の時刻を `done_since` に、doneの投稿が名指すPR/MRのURLを `change_url` に入れる。
1件の詳細は、openのまま `agent:done` のissueで `done` を真にしなければならない（MUST）。closeされたissueでは偽にする。

#### Scenario: 完了のissueの要素
- **WHEN** PRを作ってdoneになったopenのissueがある状態で、一覧を求める
- **THEN** そのissueの要素は `state` が `done` で、`done_since` と `change_url` を持つ
