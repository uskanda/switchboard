# web Specification

## Purpose

受信箱をブラウザで見て、返答できるようにする。デスクトップとスマホの両方で使える。WUIは核とは別のモジュールで、フォージのトークンを持たず、核の `switchboard inbox serve` を通してだけ受信箱を扱う。

## Requirements

### Requirement: 待ち受けのアドレス
`<switchboard-web>` は、`<web.yaml>` の `<listen>` に書いたアドレスで待たなければならない（MUST）。既定のアドレスは持たない。
`<listen>` が無いとき、ホストが空のとき、`0.0.0.0` か `::` のとき、`<switchboard-web>` は起動を拒まなければならない（MUST）。
WUIの設定は `<web.yaml>` に置き、核の `config.yaml` には書かない。

#### Scenario: すべてのインターフェースで待とうとする
- **WHEN** `<listen>` に `0.0.0.0:8783` を書いて `<switchboard-web>` を起動する
- **THEN** 起動せず、LANのアドレスを明示するよう報告する

### Requirement: 核とのつなぎ
`<switchboard-web>` は、受信箱の一覧、1件の詳細、返答を、`<inboxapi>` を通して `switchboard inbox serve` に求めなければならない（MUST）。
`<switchboard-web>` は、フォージのトークンを持ってはならず、フォージのAPIを直接呼んではならない（MUST NOT）。
ソケットに届かないとき、WUIは、受信箱のサービスに届かないことを画面に示す。

#### Scenario: 受信箱のサービスが止まっている
- **WHEN** `switchboard inbox serve` が動いていない状態で、一覧の画面を開く
- **THEN** 受信箱のサービスに届かないことが表示される

### Requirement: 画面
一覧の画面は、`switchboard inbox` と同じ項目を、待ちが始まった古い順に表示しなければならない（MUST）。届かなかったフォージがあれば、そのことも示す。
1件の画面は、質問の全文、拒否された操作、フォージのissueへのリンク、返答の欄を表示しなければならない（MUST）。
画面は、デスクトップとスマホのどちらの幅でも、横へのスクロール無しで使えなければならない（MUST）。
issueから来た文字列（タイトル、質問）は、テキストとして表示し、HTMLとして解釈させてはならない（MUST NOT）。
画面の静的ファイルは、すべて `<switchboard-web>` に埋め込み、外のCDNから読み込んではならない（MUST NOT）。

#### Scenario: スマホの幅で開く
- **WHEN** 幅360pxのブラウザで一覧と1件の画面を開く
- **THEN** 横へのスクロール無しで、返答を書いて送れる

#### Scenario: タイトルにHTMLが含まれる
- **WHEN** タイトルが `<script>` を含む待ちのissueを一覧に出す
- **THEN** その文字列がそのまま表示され、スクリプトは動かない

### Requirement: 返答
1件の画面から送った返答を、`<switchboard-web>` は `<inboxapi>` で投稿しなければならない（MUST）。待ちでないと返されたら、そのことを画面に示す。
返答のPOSTについて、`<switchboard-web>` は、`Origin` と `Host` が `<listen>` のアドレスと一致することを確かめなければならない（MUST）。一致しないか、`Origin` が無いときは、投稿せずに403を返す。

#### Scenario: 外のWebページから送られる
- **WHEN** 別のオリジンのWebページが、WUIの返答のURLへPOSTを送る
- **THEN** 403が返り、issueには何も投稿されない

#### Scenario: 別の名前で届く
- **WHEN** `Host` が `<listen>` と違う名前の返答のPOSTが届く
- **THEN** 403が返り、issueには何も投稿されない

### Requirement: 認証
当面、WUIは利用者を認証しない。LANの中から届く要求は、すべて利用者のものとして扱う。認証が無くても、返答のPOSTでの `Origin` と `Host` の検査は省いてはならない（MUST NOT）。
この扱いは、ADR-036と、認証を課題として残したissueに記録する。

#### Scenario: LANの別の機器から開く
- **WHEN** LANの中のブラウザで、WUIのアドレスを開く
- **THEN** ログインを求めずに一覧が表示される

### Requirement: 設定の画面
WUIは、ヘッダから開ける設定の画面を持たなければならない（MUST）。
設定の画面は、全体の指示と、設定にあるプロジェクトごとの指示の欄を並べ、欄ごとに保存できなければならない（MUST）。
設定の画面は、指示のファイルがどの端末のものかを表示しなければならない（MUST）。
保存の誤り（大きさの上限、秘密らしいもの、受信箱のサービスに届かない）は、どの欄の誤りかが分かる形で画面に示す。

#### Scenario: プロジェクトの指示を保存する
- **WHEN** 設定の画面で `gh:uskanda/switchboard` の欄に指示を書いて保存する
- **THEN** `<inboxapi>` を通して保存され、画面には端末 `ubuntu-a` の指示として保存されたことが示される

#### Scenario: トークンを貼り付けた
- **WHEN** 設定の画面で、トークンを含む指示を保存しようとする
- **THEN** 保存されず、その欄に、秘密らしいものを含むと示される

### Requirement: 追加の指示の保存の検査
指示の保存のPOSTについて、`<switchboard-web>` は、本文がJSONであることを確かめなければならない（MUST）。
同じPOSTについて、`<switchboard-web>` は、`Origin` と `Host` が `<listen>` のアドレスと一致することを確かめなければならない（MUST）。一致しないか、`Origin` が無いときは、保存せずに403を返す。

#### Scenario: 外のWebページから送られる
- **WHEN** 別のオリジンのWebページが、WUIの指示の保存のURLへPOSTを送る
- **THEN** 403が返り、指示のファイルは変わらない

### Requirement: 計画の待ちの表示
一覧の画面と1件の画面は、待ちの理由 `plan-questions` と `plan` を、ほかの理由と同じく理由として表示しなければならない（MUST）。
理由が `plan` のとき、1件の画面は、返答の欄の近くに、承認の語を送れば承認になることを示さなければならない（MUST）。

#### Scenario: 計画の承認待ちを開く
- **WHEN** 理由が `plan` の待ちのissueを1件の画面で開く
- **THEN** 計画の全文と、`/ok` を送れば承認になることが表示される
