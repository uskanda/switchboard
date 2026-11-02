## ADDED Requirements

### Requirement: 完了の一覧
`switchboard inbox` は、`agent:done` が付いたopenのissueを設定済みのプロジェクトから集め、完了の節に表示しなければならない（MUST）。節の見出しは `Done (waiting for close)` とし、順番待ちの節の下に置く。
各行は、行番号、issue、端末の名前、完了からの経過時間、PR/MRのURLを持たなければならない（MUST）。端末の名前と完了の時刻は、エージェントの最新のdoneの投稿のマーカーからだけ取る。PR/MRのURLは、その投稿が名指すものとする。
行は、完了した時刻の古い順に並べる。行番号は、待ちの行に続けて振る。
closeされたissueは、一覧に出してはならない（MUST NOT）。

#### Scenario: 完了のissueが残る
- **WHEN** 待ちのissueが2件と、openのまま `agent:done` のissueが1件ある状態で `switchboard inbox` を実行する
- **THEN** 完了の節に、行番号3でそのissueとPRのURLが出る

#### Scenario: closeで消える
- **WHEN** 完了の節に出ていたissueを、マスターがcloseする
- **THEN** 次の `switchboard inbox` で、そのissueは出ない

#### Scenario: 行番号で返答する
- **WHEN** 完了の行の行番号3を指定して `switchboard reply 3 "コンフリクトを直してください"` を実行する
- **THEN** そのissueに返答がコメントされる

## MODIFIED Requirements

### Requirement: 返答前の確認
投稿の前に、`switchboard reply` はissueを取り直し、openで `agent:waiting` か `agent:done` が付いていることを確かめなければならない（MUST）。
どちらも無いとき、`--force` が無ければ、`switchboard reply` は投稿せずに理由を報告しなければならない（MUST）。

#### Scenario: 既に再開していた
- **WHEN** 一覧を表示した後にissueが `agent:running` になり、その後で `switchboard reply 1 "..."` を実行する
- **THEN** `switchboard reply` は投稿せず、issueが待ちと完了のどちらの状態でもないと報告する

#### Scenario: 完了のissueへの返答
- **WHEN** openのまま `agent:done` のissueに `switchboard reply` で返答する
- **THEN** 返答はコメントされる

#### Scenario: closeされたissueへの返答
- **WHEN** closeされた `agent:done` のissueに、`--force` を付けずに返答する
- **THEN** `switchboard reply` は投稿せず、closeされていると報告する
