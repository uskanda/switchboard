# Spec Delta

## MODIFIED Requirements

### Requirement: 待ちの通知
エージェントが待ちのコメントを投稿したとき、エージェントは通知を1通送らなければならない（MUST）。待ちの理由（question、permission、error、verify、plan-questions、plan）は問わない。
通知には、プロジェクト、issue番号、issueのタイトル、待ちの理由、質問の抜粋、issueのURLを載せる。`<notify.web_url>` があれば、WUIのURLも載せる。
質問の抜粋は、エージェントが投稿した質問から取り、長ければ切り詰める。計画のコメント（`plan`）では、計画の冒頭を抜粋とする。通知の本文は、書式を解釈させない平文で送る。
二重投稿の防止によって投稿しなかったときは、通知を送ってはならない（MUST NOT）。

#### Scenario: 質問を残して待ちになる
- **WHEN** 実行が質問を残して終わり、エージェントが待ちのコメントを投稿する
- **THEN** Telegramに1通届き、プロジェクト、issue番号、タイトル、理由のquestion、質問の抜粋、issueのURLが載っている

#### Scenario: 既に投稿済みだった
- **WHEN** 再起動後の回復で、同じ実行の待ちのコメントが既に投稿されていると分かる
- **THEN** 通知は送られない

#### Scenario: 計画の承認待ち
- **WHEN** 計画の実行が計画を宣言し、エージェントが計画のコメントを投稿する
- **THEN** Telegramに1通届き、理由のplanと計画の冒頭が載っている
