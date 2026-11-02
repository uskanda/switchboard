# Design

## Context

動機はproposal.mdにある。今のループは、`interval()` が返す間隔のtickerで `Poll` を呼び、`Poll` は担当の全プロジェクトを取り直す。`interval()` は、中継につながっていれば `reconcile_interval` を、そうでなければ `poll_interval` を返す。中継の接続と切断では、`wake` でループを起こし、全プロジェクトを1回照合する。

## Goals / Non-Goals

**Goals:**

- `relay.hooks` に載っていないプロジェクトについて、中継の有無によらず、更新から検知までを `poll_interval` 以内にする

**Non-Goals:**

- 中継を別のサーバーに移したときの設定の形を決めること

## Decisions

- `config.Relay` に `Covers(ref string) bool` を足す。`relay.hooks` の `project` と、プロジェクトの `ref` を、大文字と小文字を区別せずに比べる。フォージのパスは大文字と小文字を区別しないためである
- `interval()` は、中継につながっていて、かつ担当の全プロジェクトが `Covers` のときだけ `reconcile_interval` を返す。それ以外は `poll_interval` を返す
- ループは、tickerが鳴るたびに、取り直すプロジェクトを選ぶ。中継につながっていなければ全プロジェクトを選ぶ。つながっていれば、`Covers` でないプロジェクトを選び、最後の照合から `reconcile_interval` が過ぎていれば `Covers` のプロジェクトも選ぶ。最後の照合の時刻は、全プロジェクトを取り直したときに記録する
- `wake`（接続と切断）のときと、起動の直後は、今までどおり全プロジェクトを取り直す
- `Poll` は今までどおり全プロジェクトを取り直す関数として残し、選んだプロジェクトだけを取り直す関数を中に作る。どちらも同じロックで直列にする

案Bは採らない。案Bは、中継が扱うプロジェクトを `agent.relay` の中に書かせる形である（grilling.md）。

## Risks / Trade-offs

- ［中継を別のサーバーに移すと、エージェントの設定から `relay.hooks` が無くなる］→全プロジェクトが `poll_interval` のポーリングに戻る。ポーリングが増えるだけで、取りこぼしは起きない。設定の形は、移すときに決め直す
