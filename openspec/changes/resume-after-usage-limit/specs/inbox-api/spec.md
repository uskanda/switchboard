## ADDED Requirements

### Requirement: 順番待ちの項目の再開の時刻
一覧の順番待ちの項目は、順番待ちのマーカーが再開の時刻を持つとき、その時刻を `queued_until` として持たなければならない（MUST）。持たないときは、項目に `queued_until` を含めない。

#### Scenario: 利用上限の順番待ち
- **WHEN** 再開の時刻が2026-11-03T18:30:00Zの、利用上限の順番待ちのissueがある
- **THEN** `GET /v1/inbox` のその項目は、`reason` が `limit`、`queued_until` が2026-11-03T18:30:00Zである
