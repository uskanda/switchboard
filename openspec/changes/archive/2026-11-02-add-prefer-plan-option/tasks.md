# Tasks

## 1. 設定（`internal/config`）

- [x] 1.1 `Agent` に `PreferPlan`（`prefer_plan`、既定はfalse）を足す。`TestPreferPlanDefaultsToFalse` で、既定がfalseであることと `true` を読めることを確かめた
- [x] 1.2 `Change` に `PreferPlan` を載せ、`changedSections` の比較から外す。再起動を求めるログの文言に `agent.prefer_plan` を足す。`TestWatcherAppliesTheAgentPreferPlan` で、値が `Change` に載り、`Ignored` に何も出ないことを確かめた

## 2. 順番待ちと読み直し（`internal/agent`）

- [x] 2.1 `queuedRun`（プロジェクト、計画の実行かどうか、入った時刻）を順番待ちの値にし、入り直すときは入った時刻を残す（design D1）。`TestQueuedIssueKeepsItsPlaceWhenItsKindChanges` で、計画から実装に変わった `#9` が入った時刻を保ち、`#10`、`#9`、`#11` の順に始まることを確かめた
- [x] 2.2 `queueOrder` で取り出す順を決める（design D2）。`TestQueueStartsInTheOrderEntered` でオフのときの順を、`TestPreferPlanStartsThePlanRunFirst` でオンのときの順を確かめた
- [x] 2.3 `errQueued` で戻ったissueの時刻を `keepPlace` で戻す（design D3）。`TestQueueOrderBreaksTiesByRefAndKeepPlaceRestoresTheTime` で、同じ時刻のときのrefの順と、時刻が戻ることを単体で確かめた
- [x] 2.4 `Reload` と `applyProjects` で、読み直した `agent.prefer_plan` を `agent.max_parallel` と一緒に反映する（design D4）。`TestReloadTurnsPreferPlanOn` で、読み直しでオンにすると次の取り出しから `#9` が先になることを確かめた

## 3. 文書

- [x] 3.1 `docs/DECISIONS.md` に、取り出す順と `agent.prefer_plan` のADRを足す。mainとのADRの番号の重なりを2回直し、ADR-046に改めた
- [x] 3.2 `README.md` の設定の説明と例、`docs/DESIGN.md` のschedulerの項と例に、`prefer_plan` と取り出す順を書き足す
- [x] 3.3 `openspec/specs/config/spec.md` の「設定の読み直し」と「projects以外の変化」に `<agent.prefer_plan>` を加える

## 4. 全体の確認

- [x] 4.1 `make verify` を通す。PR #79のhead（a541c89）でCIの `verify` が通った
- [ ] 4.2 `errQueued` でプロジェクトの上限から順番待ちへ戻ったissueが、入った時刻を保つことを結合テストか実機で確かめる。PR #79は単体のテストだけで確かめた
