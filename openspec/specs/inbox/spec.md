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
投稿の前に、`switchboard reply` はissueを取り直し、`agent:waiting` が付いていることを確かめなければならない（MUST）。
`agent:waiting` が無いとき、`--force` が無ければ、`switchboard reply` は投稿せずに理由を報告しなければならない（MUST）。

#### Scenario: 既に再開していた
- **WHEN** 一覧を表示した後にissueが `agent:running` になり、その後で `switchboard reply 1 "..."` を実行する
- **THEN** `switchboard reply` は投稿せず、issueは待ちの状態ではないと報告する

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
