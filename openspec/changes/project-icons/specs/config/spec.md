## ADDED Requirements

### Requirement: プロジェクトのアイコンの書き方
`switchboard` は、プロジェクトの `icon` に、次の3つの形のどれかを受け付けなければならない（MUST）。
- `https://` で始まる画像のURL
- `/` か `~/` で始まる、端末の画像ファイルのパス
- 絵文字1文字（書記素1つ）

`icon` が無いとき、プロジェクトのアイコンはフォージから取る。

#### Scenario: 絵文字を書く
- **WHEN** `projects` のプロジェクトの `icon` に、ロケットの絵文字（U+1F680）を1文字だけ書く
- **THEN** 設定は受け付けられ、そのプロジェクトのアイコンはその絵文字として扱われる

#### Scenario: 画像のファイルを書く
- **WHEN** `projects` のプロジェクトに `icon: ~/icons/sb.png` がある
- **THEN** 設定は受け付けられ、アイコンはホームディレクトリの下の `icons/sb.png` から読まれる

### Requirement: 書き方に合わないアイコン
プロジェクトの `icon` が、画像のURLでもファイルのパスでもなく、2つ以上の書記素から成るとする。
このとき、`switchboard` は設定の読み込みを止め、そのプロジェクトの `icon` が誤りだと報告しなければならない（MUST）。

#### Scenario: 相対パスを書く
- **WHEN** 1つ目のプロジェクトに `icon: icon.png` がある
- **THEN** コマンドは0以外の終了コードで終わり、`projects[<n>].icon` の書き方が誤りだと報告する

#### Scenario: httpのURLを書く
- **WHEN** 1つ目のプロジェクトに `icon: http://example.com/a.png` がある
- **THEN** コマンドは0以外の終了コードで終わり、`projects[<n>].icon` の書き方が誤りだと報告する
