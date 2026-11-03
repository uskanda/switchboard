## MODIFIED Requirements

### Requirement: 異常な終了
次のどれかのとき、エージェントは実行を `waiting(reason=error)` として扱わなければならない（MUST）。
終了コードが0以外のとき、resultの `is_error` が真のとき、`structured_output` が無いかスキーマに合わないときが該当する。
ただし、アダプタが利用上限と判定した実行は、直す実行を除いて、待ちにせず順番待ちへ戻す（agent-runの要件「上限で止まった実行の順番待ちへの戻し」）。

#### Scenario: 最大ターン数に達した
- **WHEN** 実行が `max_turns` に達して終わる
- **THEN** issueは待ちになり、コメントに理由が書かれる

#### Scenario: 利用上限で終わった
- **WHEN** 実装の実行が、`is_error` が真で `api_error_status=429` のresultで終わる
- **THEN** issueは待ちにならず、`agent:ready` と順番待ちのコメントに戻る

## ADDED Requirements

### Requirement: 直す実行の利用上限
直す実行が利用上限で終わったとき、エージェントは、その実行を `waiting(reason=error)` で終えなければならない（MUST）。本文には、利用上限で止まったことと再開の時刻を書き、マーカーに `until` を付ける。
直す実行が利用上限で終わったとき、エージェントは、端末を再開の時刻まで止めなければならない（MUST）。
利用上限で終わった直す実行を、エージェントは自動で直す回数に数えてはならない（MUST NOT）。

#### Scenario: CIを直す実行が上限に当たる
- **WHEN** `<agent.max_fix_attempts>` が2で、CIを直す1回目の実行が利用上限で終わる
- **THEN** issueは `waiting(reason=error)` になり、本文に利用上限と再開の時刻が出る。その後の自動で直す回数は0回から数える

### Requirement: マーカーの再開の時刻
順番待ちのマーカーと、利用上限で終わった直す実行の待ちのマーカーは、再開の時刻を `until=<RFC3339>` として持たなければならない（MUST）。
上限で止まった実行の順番待ちのマーカーは、その実行のIDを `run=` として持たなければならない（MUST）。

#### Scenario: 上限の順番待ちのマーカー
- **WHEN** 実行 `r1` が利用上限で終わり、再開の時刻が2026-11-03T18:30:00Zである
- **THEN** 順番待ちのコメントのマーカーは `state=queued reason=limit run=r1 until=2026-11-03T18:30:00Z` を含む
