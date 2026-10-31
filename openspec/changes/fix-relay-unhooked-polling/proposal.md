# Proposal

## Why

中継につながっている間、エージェントは照合の間隔を `poll_interval` から `reconcile_interval`（既定10分）に延ばす。この延ばした間隔が、担当する全プロジェクトに使われている。中継からイベントが届くのは、`relay.hooks` にwebhookを登録したプロジェクトだけである。そのため、webhookの無いプロジェクトは、10分に1回しか見られない。usknasでは、`gh:uskanda/switchboard` #17に `agent:ready` を付けてから評価までに、約4分かかった。中継につながったのが13:16:48で、次の照合が13:26:48だった。

## What Changes

- 中継につながっている間も、`relay.hooks` に載っていないプロジェクトは、`poll_interval` ごとにポーリングする
- `relay.hooks` に載っているプロジェクトは、今までどおり `reconcile_interval` ごとに照合する
- ループの間隔を `reconcile_interval` にするのは、担当の全プロジェクトが `relay.hooks` に載っているときだけにする。1つでも載っていなければ、ループは `poll_interval` で回す。そのとき、載っているプロジェクトは、照合の時期が来たときだけ取り直す
- 中継につながったときと切れたときに全プロジェクトを1回照合する動きは、変えない

## Capabilities

### New Capabilities

- なし

### Modified Capabilities

- `agent-run`: 中継につながっている間の照合を、`relay.hooks` に載っているプロジェクトに限る

## Impact

- `internal/agent` の、ポーリングの間隔の決め方とループ
- `internal/config` に、プロジェクトが `relay.hooks` に載っているかを引く関数を足す
- README.mdの中継の節で、照合の間隔を使うプロジェクトの範囲を書き直す
