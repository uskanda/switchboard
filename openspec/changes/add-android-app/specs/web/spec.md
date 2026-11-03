# Spec Delta

## ADDED Requirements

### Requirement: アプリの登録と承認の取り次ぎ
`<switchboard-web>` は、アプリの登録のPOSTと計画の承認のPOSTを、`<inboxapi>` で受信箱のサービスに取り次がなければならない（MUST）。
どちらのPOSTについても、`<switchboard-web>` は、返答のPOSTと同じ検査をしなければならない（MUST）。本文がJSONであることと、`Origin` と `Host` が `<listen>` のアドレスと一致することである。
どちらかに当たらないPOSTを、`<switchboard-web>` は受信箱のサービスに渡してはならない（MUST NOT）。

#### Scenario: アプリから登録する
- **WHEN** アプリが、`Origin` に `<listen>` のオリジンを付けて、JSONの登録のPOSTを送る
- **THEN** トークンが受信箱のサービスに登録される

#### Scenario: Originの無い承認
- **WHEN** `Origin` の無い承認のPOSTが届く
- **THEN** 403が返り、issueにコメントは付かない

### Requirement: 問い合わせで開くマージの確認のダイアログ
1件の画面のURLに `<action=merge>` の問い合わせがあるとき、WUIは、そのissueの詳細を読んだあとでマージの確認のダイアログを開かなければならない（MUST）。ダイアログは、マージのボタンで開くものと同じにする。
そのissueにマージのボタンを出さないとき、WUIは、ダイアログを開かずに、マージできないことを誤りのトーストで示さなければならない（MUST）。
ダイアログを閉じたとき、WUIは、URLから問い合わせを外さなければならない（MUST）。

#### Scenario: 通知から開く
- **WHEN** 完了の `o/r#12` の1件の画面を、`<action=merge>` の問い合わせ付きで開く
- **THEN** マージの確認のダイアログが開き、「マージする」を押すまでマージは送られない

#### Scenario: もう閉じられていた
- **WHEN** 既に閉じられたissueの1件の画面を、`<action=merge>` の問い合わせ付きで開く
- **THEN** ダイアログは開かず、誤りのトーストが出る
