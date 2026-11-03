# Spec Delta

## ADDED Requirements

### Requirement: ファビコン
待ちのissueが1件以上ある間、WUIは、待ちの印の付いたファビコンを出さなければならない（MUST）。
待ちのissueが無い間、WUIは、通常のファビコンを出さなければならない（MUST）。
WUIは、待ちのissueを、タブのタイトルの`(N)`と同じ基準で数えなければならない（MUST）。
WUIは、ファビコンを同じオリジンの静的なファイルで配り、CSPを緩めてはならない（MUST NOT）。

#### Scenario: 待ちが1件になる
- **WHEN** 待ちの無い一覧のあとに、待ちのissueを1件含む一覧が届く
- **THEN** ファビコンは待ちの印の付いたものに変わる

#### Scenario: 待ちが無くなる
- **WHEN** 待ちのissueを含む一覧のあとに、待ちの無い一覧が届く
- **THEN** ファビコンは通常のものに戻る

### Requirement: ホーム画面への追加
WUIは、ホーム画面に追加するためのmanifestを配らなければならない（MUST）。manifestには、name、short_name、start_url、display（standalone）、テーマの色、背景の色、192pxと512pxのアイコンを書く。
WUIは、180pxのapple-touch-iconを配らなければならない（MUST）。
WUIは、manifestをJSONの型で返さなければならない（MUST）。

#### Scenario: manifestを読む
- **WHEN** ブラウザが`index.html`の指すmanifestを読む
- **THEN** 応答の型は`application/json`で、本文はmanifestである

#### Scenario: iOSでホーム画面に追加する
- **WHEN** iPhoneのSafariでWUIをホーム画面に追加する
- **THEN** アイコンは180pxのapple-touch-iconになる

### Requirement: 一覧の時点の表示
自動更新がliveでなく、一覧がある間、WUIは、ヘッダの自動更新の状態の後ろに「HH:MM時点の一覧」を出さなければならない（MUST）。
WUIは、時点の時刻に、一覧の`fetched_at`と、自動更新が最後にliveだった時刻のうち遅いほうを使わなければならない（MUST）。
時点の時刻が今日でないとき、WUIは、時刻を「M/D HH:MM」と出さなければならない（MUST）。
自動更新がliveの間、WUIは、一覧の時点を出してはならない（MUST NOT）。

#### Scenario: 受信箱のサービスが止まる
- **WHEN** 自動更新がliveの状態から、ストリームが切れる
- **THEN** ヘッダに、切れた時刻の「HH:MM時点の一覧」が出る

#### Scenario: 再接続する
- **WHEN** 切れたあとに再び一覧が届き、自動更新がliveに戻る
- **THEN** 一覧の時点の表示は消える

#### Scenario: 自動更新が無い
- **WHEN** ブラウザにEventSourceが無く、`/api/inbox`の一覧だけを読んだ
- **THEN** ヘッダに、その一覧の`fetched_at`の「HH:MM時点の一覧」が出る

#### Scenario: 前日の一覧
- **WHEN** 時点の時刻が前日である
- **THEN** 時刻は「M/D HH:MM」の形で出る
