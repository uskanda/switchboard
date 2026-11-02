## ADDED Requirements

### Requirement: マージのボタン
完了のissueの `change_number` と `change_sha` があるとき、WUIは、一覧の完了の行と1件の表示に「マージ」のボタンを出さなければならない（MUST）。一覧の行では、ボタンをPR/MRのリンクの隣で、行のリンクの外に置く。
`change_number` か `change_sha` が無いとき、WUIは、マージのボタンを出してはならない（MUST NOT）。
WUIは、GitHubのPR/MRを `PR #N`、GitLabのPR/MRを `MR !N` と表記しなければならない（MUST）。

#### Scenario: 完了の行のボタン
- **WHEN** `change_number` が73で `change_sha` のある完了の行を、一覧で見る
- **THEN** PRのリンクの隣にマージのボタンが出て、ボタンを押しても1件の表示には移らない

#### Scenario: 古い完了
- **WHEN** `change_number` の無い完了のissueを選ぶ
- **THEN** 1件の表示にマージのボタンは出ない

### Requirement: マージの確認のダイアログ
マージのボタンを押したとき、WUIは、送る前に確認のダイアログを開かなければならない（MUST）。ダイアログには、マージするPR/MRの名前、取り消せないことの1文、PR/MRへのリンク、「やめる」と「マージする」のボタンを出す。
ダイアログで「やめる」を押したとき、WUIは、マージを送ってはならない（MUST NOT）。
送っている間、WUIは、ダイアログの両方のボタンを押せなくし、送信中であることを示さなければならない（MUST）。

#### Scenario: やめる
- **WHEN** マージのボタンを押し、ダイアログで「やめる」を押す
- **THEN** ダイアログが閉じ、マージの要求は送られない

#### Scenario: マージする
- **WHEN** マージのボタンを押し、ダイアログで「マージする」を押す
- **THEN** 行か詳細が示すissueと `change_sha` を付けて、マージの要求が送られる

### Requirement: マージの結果の表示
マージの結果が返ったとき、WUIは、ダイアログを閉じ、結果をトーストで表示しなければならない（MUST）。成功は成功のトースト、断られたときは誤りのトーストとし、誤りのトーストは理由を2行目に出す。
WUIは、マージの誤りをダイアログの中に残してはならない（MUST NOT）。
権限が無いと断られたとき、誤りのトーストは、受信箱のトークンに要る権限を名指さなければならない（MUST）。

#### Scenario: マージできた
- **WHEN** ダイアログで「マージする」を押し、マージが成功する
- **THEN** ダイアログが閉じ、`PR #73をマージしました` の成功のトーストが出る

#### Scenario: 画面が古い
- **WHEN** 画面を読んだ後にPRへコミットが足され、マージが `stale` で断られる
- **THEN** ダイアログが閉じ、開き直すよう促す誤りのトーストが出る

#### Scenario: 権限が無い
- **WHEN** マージが `forbidden` で断られる
- **THEN** 誤りのトーストに、GitHubの `Pull requests` と `Contents` の書き込みと、GitLabの `api` のスコープが出る

### Requirement: マージのPOSTの検査
マージのPOSTについて、`<switchboard-web>` は、返答のPOSTと同じ検査をしなければならない（MUST）。本文がJSONであることと、`Origin` と `Host` が `<listen>` のアドレスと一致することである。
どちらかに当たらないPOSTを、`<switchboard-web>` は受信箱のサービスに渡してはならない（MUST NOT）。

#### Scenario: 外のWebページから送られる
- **WHEN** 別のオリジンのWebページが、WUIのマージのURLへPOSTを送る
- **THEN** 403が返り、PR/MRはマージされない
