## ADDED Requirements

### Requirement: 通知の設定
設定は、`<notify.telegram>` に、`chat_id` と `token_ref` を持てなければならない（MUST）。片方だけのときは、設定の誤りとして拒む。
`<notify.web_url>` は、WUIのURLで、`http` か `https` の絶対URLでなければならない（MUST）。無くてもよい。
`<notify>` の節が無い設定は、今までどおり読めなければならない（MUST）。

#### Scenario: chat_idだけがある
- **WHEN** `<notify.telegram>` に `chat_id` だけがあり、`token_ref` が無い設定を読む
- **THEN** 設定の誤りとして報告される

#### Scenario: 通知の節が無い
- **WHEN** `<notify>` の無い設定を読む
- **THEN** 誤りにならず、通知は無効になる
