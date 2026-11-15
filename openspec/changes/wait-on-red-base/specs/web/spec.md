## ADDED Requirements

### Requirement: マージのダイアログのbaseのCI
マージの前の状態にbaseの失敗したチェックがあるとき、WUIは、マージの確認のダイアログに、baseのブランチの名前と、baseのCIが失敗していることを出さなければならない（MUST）。失敗したチェックの名前は、フォージへのリンクにして並べる。
baseの失敗したチェックが無いとき、WUIは、baseのCIについて何も出してはならない（MUST NOT）。
baseのCIが失敗していても、WUIは、「マージする」を押せないようにしてはならない（MUST NOT）。
文言は、日本語と英語の両方を用意しなければならない（MUST）。

#### Scenario: mainが赤い
- **WHEN** mainの先頭のコミットでjob `verify` が失敗している状態で、完了のissueのマージのボタンを押す
- **THEN** ダイアログに「base（main）のCIが失敗しています」と、`verify` のリンクが出る。「マージする」は押せる

#### Scenario: mainが緑
- **WHEN** mainの先頭のコミットのCIが成功している状態で、マージのボタンを押す
- **THEN** ダイアログにbaseのCIの行は出ない

#### Scenario: 英語の表示
- **WHEN** 表示の言語が英語で、mainの先頭のコミットのCIが失敗している
- **THEN** ダイアログに、baseのCIの失敗を英語で書いた行が出る
