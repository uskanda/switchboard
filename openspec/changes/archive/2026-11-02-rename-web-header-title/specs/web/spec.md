## ADDED Requirements

### Requirement: ヘッダの名前
WUIのヘッダは、名前として「SwitchBoard Inbox」を表示しなければならない（MUST）。名前は一覧の画面へのリンクとする。
幅640px未満の画面では、WUIのヘッダは、名前を「SwitchBoard」に縮めて表示しなければならない（MUST）。

#### Scenario: デスクトップの幅で開く
- **WHEN** 幅1280pxのブラウザで一覧の画面を開く
- **THEN** ヘッダの見出しに「SwitchBoard Inbox」が出る

#### Scenario: スマホの幅で開く
- **WHEN** 幅360pxのブラウザで一覧の画面を開く
- **THEN** ヘッダの見出しには「SwitchBoard」だけが出る
