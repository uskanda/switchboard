## ADDED Requirements

### Requirement: プロジェクトのアイコンの画像
inboxサービスは、`GET /v1/project-icon?ref=<プロジェクトの参照>` に、そのプロジェクトのアイコンの画像を返さなければならない（MUST）。
答えには、画像の種類を表す `Content-Type` と、`Cache-Control: max-age=3600` を付ける。
プロジェクトの `icon` が画像のURLかファイルのとき、inboxサービスはそれを使わなければならない（MUST）。
`icon` が無いとき、inboxサービスは受信箱のトークンでフォージのアバターを取らなければならない（MUST）。
フォージのアバターは、GitLabではプロジェクトのアバター、それが無ければプロジェクトの属するグループのアバターとする。GitHubではリポジトリのオーナーのアバターとする。

#### Scenario: GitHubのプロジェクト
- **WHEN** `icon` の無い `gh:o/r` について `GET /v1/project-icon?ref=gh:o/r` を呼ぶ
- **THEN** オーナー `o` のアバターのPNGが `Content-Type: image/png` で返る

#### Scenario: GitLabのプロジェクトにアバターが無い
- **WHEN** アバターの無い `gl:nas/sb` について呼び、グループ `nas` にアバターがある
- **THEN** グループ `nas` のアバターが返る

#### Scenario: 設定のアイコンを優先する
- **WHEN** `gh:o/r` の `icon` に画像のURLがある
- **THEN** フォージには問い合わせず、そのURLの画像が返る

### Requirement: アイコンが無いときの答え
次のどれかのとき、inboxサービスは `GET /v1/project-icon` に404（`not_found`）を返さなければならない（MUST）。
- プロジェクトのアイコンが無い
- 画像を取れない
- `icon` が絵文字である
- 参照が設定に無いプロジェクトを指す

#### Scenario: アバターの無いプロジェクト
- **WHEN** フォージがプロジェクトのアバターもグループのアバターも返さない
- **THEN** 404が返る

### Requirement: アイコンの画像の検め
inboxサービスは、中身の先頭のバイトがPNG、JPEG、GIF、WebPのどれかである画像だけを返さなければならない（MUST）。
inboxサービスは、SVGの画像と、256KiBを超える画像を返してはならない（MUST NOT）。

#### Scenario: SVGのアバター
- **WHEN** フォージのアバターがSVGである
- **THEN** 404が返り、SVGはブラウザへ渡らない

#### Scenario: 大きすぎる画像
- **WHEN** `icon` のURLの画像が300KiBある
- **THEN** 404が返る

### Requirement: アイコンの保存
inboxサービスは、取った画像を端末の状態の置き場の下に保存しなければならない（MUST）。
保存から24時間は、inboxサービスはフォージや `icon` のURLに問い合わせず、保存した画像を返す。
画像を取れなかったとき、inboxサービスは、そのことを1時間覚え、そのあいだ問い合わせ直してはならない（MUST NOT）。

#### Scenario: 2度目の読み出し
- **WHEN** 1時間前に取った `gh:o/r` のアイコンを、もう一度読む
- **THEN** フォージには問い合わせず、保存した画像が返る

#### Scenario: 取れなかったあと
- **WHEN** 10分前にアバターを取れなかったプロジェクトのアイコンを、もう一度読む
- **THEN** フォージには問い合わせず、404が返る

### Requirement: アイコンの画像を取るときのトークン
inboxサービスは、受信箱のトークンを、フォージのAPIのホストと、GitLabのインスタンスと同じホストにだけ送らなければならない（MUST）。
inboxサービスは、GitHubのアバターの画像のホストと、`icon` のURLのホストに、トークンを送ってはならない（MUST NOT）。

#### Scenario: GitHubのアバターの画像
- **WHEN** APIとは別のホストにあるGitHubのアバターの画像を取る
- **THEN** 要求に `Authorization` のヘッダーは付かない

### Requirement: プロジェクトの絵文字の一覧
inboxサービスは、`GET /v1/project-icons` に、参照から絵文字への対応をJSONのオブジェクトで返さなければならない（MUST）。
対応には、`icon` が絵文字のプロジェクトだけを入れる。

#### Scenario: 絵文字を読む
- **WHEN** `gh:o/r` の `icon` だけが絵文字で、ほかのプロジェクトに `icon` が無い
- **THEN** 答えのオブジェクトのキーは `gh:o/r` だけで、値はその絵文字である

### Requirement: アイコンを読む公開のクライアント
`inboxapi` のクライアントは、プロジェクトのアイコンの画像と、プロジェクトの絵文字の一覧を読む関数を持たなければならない（MUST）。

#### Scenario: 画像を読む
- **WHEN** クライアントで `gh:o/r` のアイコンを読む
- **THEN** 画像の中身と `Content-Type` が返る
