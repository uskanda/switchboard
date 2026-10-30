## MODIFIED Requirements

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

## ADDED Requirements

### Requirement: 届かないフォージ
一覧を作るときに、あるフォージのAPIが失敗したとき、`switchboard inbox` は、取得できたフォージのissueだけで一覧を表示しなければならない（MUST）。
そのとき、`switchboard inbox` は、取得できなかったフォージと理由を標準エラーに書き、0以外の終了コードで終わらなければならない（MUST）。

#### Scenario: GitLabに届かない
- **WHEN** GitHubには待ちが1件あり、GitLabのAPIが接続できずに失敗する
- **THEN** 一覧にはGitHubの1件が出る。標準エラーには、GitLabに届かなかったことが書かれ、終了コードは0以外になる
