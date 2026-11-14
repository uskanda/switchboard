# Design

## Context

実行の `settings.json` は `internal/claudecode` が書く。sandboxの書き込みの範囲は、claudeの既定（worktreeと `$TMPDIR`）のままで、`filesystem` には `denyRead` だけを書いている（ADR-057）。`claude -p` の環境は `claudecode.Env` が許可リストで選ぶ。準備のコマンドと、エージェントが実行のあとに動かす検証コマンドも、同じ関数で環境を作る（`internal/agent/setup.go`、`internal/agent/run.go`）。Claudeが実行の中で動かす検証コマンドは、claudeの環境を受け継いでsandboxの外で動く。

## Goals / Non-Goals

**Goals:**

- 実装の実行と計画の実行の両方で、`go` のコマンドが前置きの変数なしに動く
- ビルドの結果を実行とプロジェクトをまたいで使い回す

**Non-Goals:**

- モジュールの取得をsandboxの中から行うこと（ネットワークは閉じたまま）
- Go以外の道具のキャッシュ（npm、pnpm、cargoなど）。必要になったら同じ形で足す

## Decisions

### 置き場所はswitchboard専用のディレクトリにする

`os.UserCacheDir()` に相当する場所（`$XDG_CACHE_HOME`、無ければ `~/.cache`）の下の `switchboard/go-build` を使う。エージェントの環境（`a.Env`）から求め、テストでは差し替えられるようにする。

- 代案：利用者自身の `~/.cache/go-build` への書き込みを許す。実行の中のコマンドの書いた結果が、利用者の普段のビルドに混ざる。sandboxの中から、sandboxの外のビルドへ影響を広げる道ができるので採らない
- 代案：実行ごとに `$TMPDIR` の下に作る。キャッシュが毎回空になり、今の回避の方法と変わらない。claudeがsandboxの中で決める `$TMPDIR` を、エージェントは起動の前に知らない
- 代案：プロジェクトごとに分ける。Goのビルドキャッシュは入力のハッシュで項目を分けるので、共有しても結果は混ざらない。分ける利点が無いので、全プロジェクトで1つにする（ユーザーが承認）

### 書き込みはsandboxの設定の `filesystem.allowWrite` で許す

`settings.json` の `sandbox.filesystem` に `allowWrite` を足し、置き場所の絶対パスを書く。この設定が `--settings` のファイルから働くかは確かめていないので、V65として実物のclaudeで確かめてから使う。確かめる内容は次の3点である。

- sandboxの中の `go build` が、`allowWrite` に書いたパスへ書ける
- ほかのパス（利用者の `~/.cache/go-build`）には書けないまま
- worktreeと `$TMPDIR` にも書けるまま

確かめた結果が違うとき（設定の名前が違う、`$TMPDIR` への書き込みが外れるなど）は、実装を止めてユーザーに戻す。`Edit(//<パス>/**)` の許可の規則では、Bashからの書き込みは許されない（V58）ので、代わりにならない。

### `GOCACHE` は許可リストではなく、エージェントが決めた値で入れる

`claudecode.Env` の許可リストに `GOCACHE` を足すと、親の環境の値（利用者のキャッシュ）がそのまま渡る。そこで、エージェントは許可リストで選んだ環境に、置き場所を `GOCACHE` として加える。親の環境の `GOCACHE` は渡さない。準備のコマンドと検証コマンドも、同じ組み立てを使う。

### 指示に書くのは1行にとどめる

sandboxの中での作業の指示（`prompt.WithSandbox`）の書き込みの行に、`GOCACHE` は設定済みで、書き換えないことを足す。計画の実行にもsandboxの指示があれば同じ行を入れる。無ければ、計画の実行の指示に同じ1行を足す。

## Risks / Trade-offs

- キャッシュの汚染：sandboxの中で書いた結果を、外の検証コマンドが読む → 検証コマンドは、Claudeが書いたworktreeのコードを、すでにsandboxの外で動かしている。キャッシュを通して新しく得る力は無い。利用者自身のキャッシュには混ざらない。SECURITYのT7にこの点を書く
- 容量：全プロジェクトのビルドの結果が1か所にたまる → Goは古い項目を自分で消す（5日使わない項目）。switchboardは消さない
- claudeの仕様変更：`allowWrite` の名前や意味が変わる → `internal/claudecode` の1か所にとどめる。V65の再現の手順をrealclaudeのテストに残す

## Migration Plan

エージェントを入れ替えれば、次の実行から置き場所を使う。置き場所は最初の実行の前に作られる。戻すときは前の版のエージェントに戻すだけで、残った置き場所は消してよい。
