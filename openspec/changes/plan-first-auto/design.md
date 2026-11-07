# Design

## Context

動機はproposal.mdのWhyを参照。決めたことはgrilling.mdにある。

今は、`agent:ready` のモードを `modeFor` が `plan_first` の真偽で `plan` か `direct` に決め、着手のマーカーの `mode` に残す。`classify` は、`mode` を持つ最新の着手から後を今の周期として読み、計画の実行、承認のあとの実装の実行、直ちに実装する実行を分ける。計画の実行は読み取りだけの権限で動き、終わったら `Restore` でworktreeを戻す。pushを拒まれたときに直す実行へ引き継ぐ仕組み（`outcome.next` と `fixAgain`）があり、同じスロットと着手のまま新しい実行を始められる。

## Goals / Non-Goals

**Goals:**
- `auto` の周期の最初の計画の実行だけが、計画の段階を省ける
- 省くまでは書き込みの権限を渡さない
- 省いたときも、計画がissueに残り、続く実装の実行がそれを名指しで読む

**Non-Goals:**
- `agent:plan`、`agent:go`、`always`、`never` の挙動を変えること
- 一度質問したあとの承認の省略
- `mode=auto` を古い版のエージェントに読ませる互換

## Decisions

### D1. `plan_first` は文字列の型にし、`UnmarshalYAML` で真偽値も読む
`config.PlanFirst`（`string`）を作り、`!!bool` の `true`/`false` を `always`/`never` に、`!!str` の `always`/`auto`/`never` をそのまま受ける。ほかは `plan_first` を名指しした誤りにする。キーが無ければ空で、`Mode()` が `never` を返す。読んだ時点で正規化するので、`sameExceptAdded` の比較も今の設定の書き戻しも変わらない。
別のキーを作る案（Q5のB）は、両方を書いたときの扱いが増えるので採らない。

### D2. モード `auto` は計画の段階として扱い、省いてよい印を `plan` に持たせる
`marker.ModeAuto` を足し、`planStage(mode)` が `plan` と `auto` を計画の段階とする。`classify` は、着手なら `mode == auto` で、返答による再開なら「今の周期に `plan-questions` も `plan` の待ちも無い `auto` の周期」で、`plan.skippable` を立てる。誤りで終わった最初の実行のあとの返答も、まだ質問していないので省いてよい。

### D3. 省ける実行だけにスキーマとsystem promptを変える
`PlanSkipSchema` は、`PlanSchema` の `status` の列挙に `implement` を足したものとする。`PlanSkipSystem` は、`PlanSystem` の言語の節の前に、省く条件、`implement` の書き方、迷ったら質問することの節を足す。利用者の追加の指示は今までどおり最後に置く。`planDeclaration` も `skippable` を受け、省けない実行の `implement` は誤りにする（スキーマで防げなくても二重に断る）。

### D4. 省いたら、計画のコメントを次の実行の着手にして引き継ぐ
`planOutcome` が `implement` を受けたら、`outcome.skip` に、題、省くことの1行、計画の本文（捨てた変更や拒まれた操作があればそれも）を書いたコメントを入れる。`execute` は `next` と同じく `skip` があれば書き戻さず、`skipAgain` に渡す。
`skipAgain` は、新しい実行のIDを作り、コメントを `state=running mode=direct reason=plan-skipped run=<新しい実行>` のマーカーで投稿する。そのあとissueを読み直し、`agent:running` のままで本文が信頼できるときだけ、`skippedRun` の実行を始める。読み直したコメントには省いたコメントが入るので、プロンプトにも計画が載る。スロットと `active` は `fixAgain` と同じく引き継ぎ、計画の実行の記録を `plan-skipped` で閉じる。
このコメントを着手とするので、`lastClaim` と `lastClaimHost` は新しい実行を指し、書き戻しの再試行も働く。`mode=direct` を持つので、以後の `classify` は直ちに実装する周期として扱い、`fixes()` と `fixSummaries()` もここを周期の始まりとみなす。
別の状態（例 `state=skipped`）を足す案は、着手を探す処理をすべて直す必要があるので採らない。

### D5. 省いたあとの実装の実行は直ちに実装する実行と同じものを渡す
`skippedRun` を `implements()` に含め、`run.md` と `implement.md`、画像の取り決め、sandboxの節、書き込みのツールと許可を渡す。system promptだけを `SkippedSystem(lang, base, planID)` にし、計画のコメントを名指しして、蒸し返さないことと計画に無い判断だけを質問することを書く。

### D6. 受信箱のAPIは文字列の項目を足し、真偽値を残す
`inboxapi` の `Project`、`IssueTarget`、`AddProjectRequest` に `plan_first_mode` を足す。`plan_first` は `always` か `auto` で真とする。追加の要求は `AddProjectRequest.Mode()` で、`plan_first_mode` が無ければ真偽値から決める。Web表示は `planStartOf` で、`plan_first_mode` が無い古いサービスには真偽値の文言を出す。

## Risks / Trade-offs

- [エージェントが省く判断を誤り、望まないものを実装する] → 基準を「聞く質問が1つも無い」に絞り、迷えば質問するよう指示する。計画はissueに残るので、done のあとに返答で直せる。必ず確かめたいissueには `agent:plan` か `always` を使う
- [省いたコメントを投稿したあとにissueを読み直せない] → 計画の実行の結果として `waiting(reason=error)` を書き戻す。以後の返答は直ちに実装する周期になる
- [`mode=auto` を古い版のエージェントが読むと `direct` として扱う] → プロジェクトを担当するのは1つの端末なので、すべての端末を同じ版にそろえる。ADRとGUIDEに書く

## Migration Plan

設定はそのまま読める。`true`/`false` を書き直す必要は無い。すべての端末のエージェントと受信箱のサービスを同じ版にそろえる。戻すときは、`plan_first: auto` を `true` か `false` に書き換えてから前の版に戻す。
