# Proposal

## Why

2026-11-14に、エージェントだけでGitHubのREST APIの枠（1時間5000回）を1時間に約7000回使い、30分足らずで使い切った（V63）。
`max_parallel: 1` のプロジェクトに順番待ちのissueが80件あり、エージェントはポーリングのたびに全件を読み直していた。1回あたりRESTで241回になる。読んだ結果は、順番待ちのままであることを確かめるだけで、何にも使われていなかった。
枠が尽きたあとも、エージェントは読みに行き続けた。同じアカウントのトークンを使う受信箱も、その間は一覧を読めなかった。

## What Changes

- ポーリングと中継の取り直しで、順番待ちのissueは、検索の結果の更新の時刻が前に読んだときから動いたときだけ評価し直す。動いていなければ読まない。枠が空いたときに実行を始める前の読み直し（`drain`）は今までどおり行う。
- フォージがAPI制限の応答を返したら、エージェントは、応答が示すリセットの時刻まで、そのフォージのクライアントでポーリング、評価、順番待ちの取り出し、CIの監視の読み取りを行わない。制限に入ったときと抜けたときに、ログを1回ずつ残す。書き戻しの再試行は、今の再試行の仕組みに任せる。
- 変化の無いポーリングで、順番待ちのissueを読まないことを確かめる回帰テストを足す。
- 測った数字をROADMAPの検証項目V64に、決定をADR-072に記録する。

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

- `agent-run`: 要件「評価のやり直し」で、順番待ちのissueは更新の時刻が動いたときだけ評価し直すことにする。API制限の間はフォージを読まない要件を足す。

## Impact

- `internal/agent/agent.go` の `pollProject`、`evaluate`、`drain`。
- `internal/agent/relay.go` の `refresh` と `interval`、`internal/agent/ciwatch.go` の `checkWatches`。
- `internal/agent` のテスト（偽のフォージで呼び出しを数える）。
- `docs/DECISIONS.md`（ADR-072）、`docs/ROADMAP.md`（V64）。
- 設定の項目、受信箱のAPI、Web表示、利用者向けの文書は変わらない。
