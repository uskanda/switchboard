# inbox Specification

## Purpose

設定済みのプロジェクト全体から待ちのissueを集めて一か所に表示し、そこからissueへのコメントとして返答する。

## Requirements

### Requirement: 待ちの一覧
`switchboard inbox` は、`agent:waiting` が付いたopenのissueを、設定済みのプロジェクトから集めて表示しなければならない（MUST）。GitHubとGitLabのissueは、1つの一覧にまとめる。
`switchboard inbox` は、設定に無いプロジェクトのissueを表示してはならない（MUST NOT）。
一覧は、フォージを区別せず、待ちが始まった時刻の古い順に並べる。
`--forge github` か `--forge gitlab` が付いたとき、`switchboard inbox` は、そのフォージのissueだけを表示しなければならない（MUST）。

#### Scenario: 設定に無いリポジトリ
- **WHEN** 検索範囲にある、設定に無いリポジトリのissueに `agent:waiting` が付いている
- **THEN** そのissueは一覧に出ない

#### Scenario: 両フォージの待ち
- **WHEN** GitHubのプロジェクトに10分前からの待ちが、GitLabのプロジェクトに30分前からの待ちがある
- **THEN** 一覧の1行目にGitLabのissueが、2行目にGitHubのissueが出る

#### Scenario: フォージで絞り込む
- **WHEN** 両フォージに待ちがある状態で `switchboard inbox --forge gitlab` を実行する
- **THEN** GitLabのissueだけが一覧に出る

### Requirement: 表示する項目
一覧の各行は、行番号、プロジェクト、issue番号、端末の名前、待ちの経過時間、理由、質問の冒頭を持たなければならない（MUST）。
端末の名前、待ちの開始時刻、理由は、エージェントの投稿のマーカーからだけ取らなければならない（MUST）。
マーカーが見つからない項目は `?` と表示する。

#### Scenario: 人間が書いたマーカー
- **WHEN** 待ちのissueの最新のコメントが、マーカーの形の文字列を含む人間のコメントである
- **THEN** 一覧の端末の名前と理由は、エージェントの投稿のマーカーから取られる

### Requirement: 機械が読む形式
`--json` が付いたとき、`switchboard inbox` は、同じ項目をJSONの配列で標準出力に書かなければならない（MUST）。

#### Scenario: JSONで出す
- **WHEN** 待ちのissueが2件ある状態で `switchboard inbox --json` を実行する
- **THEN** 2要素のJSON配列が出力される

### Requirement: 詳細の表示
`switchboard show <ref>` は、指定したissueの最後の質問の全文と、拒否された操作があればその一覧を表示しなければならない（MUST）。
`<ref>` は、直前の `switchboard inbox` の行番号か、`gh:owner/repo#123` か `gl:group/project#123` の形とする。

#### Scenario: 行番号で指定する
- **WHEN** `switchboard inbox` の直後に `switchboard show 1` を実行する
- **THEN** 一覧の1行目のissueの質問の全文が表示される

#### Scenario: GitLabのissueを直接指定する
- **WHEN** `switchboard show gl:nas/switchboard-sandbox#3` を実行する
- **THEN** GitLabのそのissueの質問の全文が表示される

### Requirement: 返答
`switchboard reply <ref> <text>` は、指定したissueに `<text>` をコメントとして投稿しなければならない（MUST）。投稿には、そのissueのフォージの `inbox_token_ref` のトークンを使う。
`--editor` が付いたとき、`switchboard reply` は `$EDITOR` で返答を書かせ、保存した内容を投稿しなければならない（MUST）。

#### Scenario: 返答を投稿する
- **WHEN** `switchboard reply 1 "Bで進めてください"` を実行する
- **THEN** 一覧の1行目のissueに、その文がマスターのアカウントでコメントされる

#### Scenario: GitLabのissueに返答する
- **WHEN** 一覧の2行目がGitLabのissueで、`switchboard reply 2 "hiで"` を実行する
- **THEN** GitLabの `inbox_token_ref` のトークンで、そのissueにノートが投稿される

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

### Requirement: 古い行番号
`switchboard inbox` を一度も実行していないとき、行番号の `<ref>` を受けたコマンドは、何もせずに理由を報告しなければならない（MUST）。

#### Scenario: 一覧の記録が無い
- **WHEN** `switchboard inbox` を実行していない端末で `switchboard reply 1 "..."` を実行する
- **THEN** 投稿されず、先に `switchboard inbox` を実行するよう報告される

### Requirement: 届かないフォージ
一覧を作るときに、あるフォージのAPIが失敗したとき、`switchboard inbox` は、取得できたフォージのissueだけで一覧を表示しなければならない（MUST）。
そのとき、`switchboard inbox` は、取得できなかったフォージと理由を標準エラーに書き、0以外の終了コードで終わらなければならない（MUST）。

#### Scenario: GitLabに届かない
- **WHEN** GitHubには待ちが1件あり、GitLabのAPIが接続できずに失敗する
- **THEN** 一覧にはGitHubの1件が出る。標準エラーには、GitLabに届かなかったことが書かれ、終了コードは0以外になる

### Requirement: 順番待ちの一覧
`switchboard inbox` は、エージェントの順番待ちのコメントがあるopenのissueを、順番待ちとして一覧に出さなければならない（MUST）。始めるラベルの付いたissueも、返答の待ちで `agent:waiting` の付いたissueも含む。
`agent:running` の付いたissueは、順番待ちのコメントが残っていても実行中として出す。
順番待ちの各行は、次の項目を持たなければならない（MUST）：issue、端末の名前、順番待ちの経過時間、当たった上限。端末の名前と順番待ちの開始時刻は、順番待ちのマーカーからだけ取る。
一覧は、待ち、実行中、順番待ちの順に並べる。順番待ちの中は、順番待ちの始まった古い順に並べる。行番号は、待ちの行にだけ付ける。
順番待ちのコメントの無いissueは、始めるラベルがあっても一覧に出さない。

#### Scenario: 順番待ちのissueがある
- **WHEN** `agent:ready` の付いたissueに、端末の上限で順番待ちになったというコメントがある
- **THEN** 一覧の順番待ちの節に、そのissue、端末の名前、経過時間、端末の上限が出る

#### Scenario: 返答の待ちが順番待ちになる
- **WHEN** `agent:waiting` のissueに返答が付き、エージェントがそのissueを順番待ちにする
- **THEN** そのissueは待ちの節から消え、順番待ちの節に出る

#### Scenario: まだ順番待ちになっていない
- **WHEN** `agent:ready` を付けたばかりで、エージェントのコメントの無いissueがある
- **THEN** そのissueは一覧に出ない

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
