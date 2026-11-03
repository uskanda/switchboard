## ADDED Requirements

### Requirement: 参照の前のプロジェクトのアイコン
WUIは、issueとプロジェクトの参照の前に、そのプロジェクトのアイコンを16pxの角丸で表示しなければならない（MUST）。
この表示は、参照とフォージのバッジを出すすべての場所に使う。新規issueのプロジェクトの選択肢には使わない。
アイコンの画像の代替テキストは空にしなければならない（MUST）。読み上げは参照の文字に任せる。
追加の指示の全体の欄のように、参照の無い場所には、アイコンを表示してはならない（MUST NOT）。

#### Scenario: 一覧の行
- **WHEN** 一覧に`gh:o/r#12`のissueがある
- **THEN** 行の`o/r#12`の前に、`/api/project-icon?ref=gh%3Ao%2Fr`を読む代替テキストの空の画像が置かれる

#### Scenario: 1件の表示の見出し
- **WHEN** `gl:nas/sb#3`のissueを開く
- **THEN** 見出しの`nas/sb#3`の前に、`gl:nas/sb`のアイコンが置かれる

### Requirement: アイコンが無いときの頭文字
アイコンの画像を読めないとき、WUIは、画像の代わりに、プロジェクト名の頭文字を出す同じ大きさの丸を表示しなければならない（MUST）。
丸の地は`muted`とし、色でプロジェクトを区別させてはならない（MUST NOT）。

#### Scenario: 画像の読み込みに失敗する
- **WHEN** `gh:o/sandbox`のアイコンの画像が404で読めない
- **THEN** 参照の前に、頭文字「S」の丸が表示される

### Requirement: 設定の絵文字のアイコン
プロジェクトの`icon`が絵文字のとき、WUIは、画像を読まずに、頭文字の丸の場所にその絵文字を表示しなければならない（MUST）。
WUIは、絵文字の一覧をページを開いたときに1回だけ読み、すべてのアイコンで共有しなければならない（MUST）。

#### Scenario: 絵文字のプロジェクト
- **WHEN** `/api/project-icons`が`gh:o/r`に絵文字を返す
- **THEN** `o/r#12`の前には、画像ではなくその絵文字が表示される

### Requirement: アイコンの画像の受け渡し
switchboard-webは、`/api/project-icon`と`/api/project-icons`のGETを、inboxサービスの同じ名前のエンドポイントへ渡さなければならない（MUST）。
画像を返すとき、switchboard-webは、inboxサービスの`Content-Type`と`Cache-Control`を保たなければならない（MUST）。
画像の答えにも、switchboard-webは`X-Content-Type-Options: nosniff`を付けなければならない（MUST）。
switchboard-webは、アイコンのためにCSP（`default-src 'self'`）を緩めてはならない（MUST NOT）。

#### Scenario: 画像を返す
- **WHEN** ブラウザが`/api/project-icon?ref=gh:o/r`を読む
- **THEN** inboxサービスの画像が`Content-Type: image/png`で返り、CSPは他の答えと同じである

#### Scenario: アイコンが無い
- **WHEN** inboxサービスが`/v1/project-icon`に404を返す
- **THEN** switchboard-webも404を返す
