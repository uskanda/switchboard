# notify Specification

## Purpose

issueが待ちになったときと完了したときに、エージェントが利用者のTelegramへ知らせる。通知は副次的な経路で、送れなくても処理を止めない。

## Requirements

### Requirement: 通知の宛先
設定に `<notify.telegram>` があるとき、エージェントは、通知をTelegramのBot APIの `sendMessage` で `chat_id` に送らなければならない（MUST）。botのトークンは、`token_ref` から `secret.Resolver` で読む。
設定に `<notify.telegram>` が無いとき、エージェントは通知を送らず、今までどおり動かなければならない（MUST）。
botのトークンを、ログ、監査ログ、エラーの文に出してはならない（MUST NOT）。

#### Scenario: 通知の設定が無い
- **WHEN** `<notify>` の無い設定でエージェントを動かし、issueが待ちになる
- **THEN** 通知は送られず、待ちのコメントとラベルは今までどおり付く

### Requirement: 待ちの通知
エージェントが待ちのコメントを投稿したとき、エージェントは通知を1通送らなければならない（MUST）。待ちの理由（question、permission、error、verify）は問わない。
通知には、プロジェクト、issue番号、issueのタイトル、待ちの理由、質問の抜粋、issueのURLを載せる。`<notify.web_url>` があれば、WUIのURLも載せる。
質問の抜粋は、エージェントが投稿した質問から取り、長ければ切り詰める。通知の本文は、書式を解釈させない平文で送る。
二重投稿の防止によって投稿しなかったときは、通知を送ってはならない（MUST NOT）。

#### Scenario: 質問を残して待ちになる
- **WHEN** 実行が質問を残して終わり、エージェントが待ちのコメントを投稿する
- **THEN** Telegramに1通届き、プロジェクト、issue番号、タイトル、理由のquestion、質問の抜粋、issueのURLが載っている

#### Scenario: 既に投稿済みだった
- **WHEN** 再起動後の回復で、同じ実行の待ちのコメントが既に投稿されていると分かる
- **THEN** 通知は送られない

### Requirement: 完了の通知
エージェントが完了のコメントを投稿したとき、エージェントは通知を1通送らなければならない（MUST）。
通知には、プロジェクト、issue番号、作ったPRかMRのURL、issueのURLを載せる。

#### Scenario: PRを作って完了する
- **WHEN** 実行が完了し、エージェントがdraftのPRを作って完了のコメントを投稿する
- **THEN** Telegramに1通届き、PRのURLとissueのURLが載っている

### Requirement: 送れなかったとき
通知が失敗したとき、エージェントは、実行、コメント、ラベルの処理を止めてはならない（MUST NOT）。再送もしない。
そのとき、エージェントは監査ログに `<notify.failed>` を、issue、通知の種類、理由とともに残さなければならない（MUST）。
1回の送信は、10秒で打ち切る。

#### Scenario: Telegramに届かない
- **WHEN** Telegramへの接続が失敗する状態で、issueが待ちになる
- **THEN** issueには待ちのコメントと `agent:waiting` が付き、監査ログに `<notify.failed>` が残る
