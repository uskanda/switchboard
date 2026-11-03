# Proposal

## Why

UIを変えた実装では、変わった画面をissueの上で確かめたい。今のswitchboardは、実装の実行にだけ指示を足す手段が無い。`run.md` は直す実行にも入るので、「実装のときだけ撮る」を書けない。画像を置く場所の取り決めと、issueへの添付の仕組みも無い。GitHubにはコメントへ画像を付ける公開のREST APIが無く、`gh` の `--attach` だけが手で付けたときと同じ形で付けられる。

## What Changes

- 指示に3つめの種類「実装の実行だけの指示」（`implement.md`。全体用とプロジェクト用）を足す。入るのは直接の実装と承認のあとの実装だけで、計画の実行と直す実行には入らない
- `implement.md` の保存の検査、監査ログ、CLI、受信箱のAPIの `kind`、設定の画面の節は、今の2種類と同じ作りにする。CLIは `switchboard instructions edit --implement` とする。`run.started` には `implement.md` のハッシュを別の鍵で残す
- 直接の実装と承認のあとの実装のsystem promptに、画像の置き場所 `.switchboard/screenshots/` の取り決めの節をいつも入れる
- 実行の前に、置き場所を `.git/info/exclude` に入れ、前の実行の残りを消す。doneで終わったら、条件に合う画像をrunのディレクトリへ移す
- doneを書き戻したあと、switchboard自身が画像だけのコメントを1件足す。GitHubは `gh issue comment --attach`、GitLabはuploads APIで付ける。失敗したら理由だけのコメントを付け、doneは保つ
- `switchboard doctor` で、GitHubのプロジェクトがあるときに `gh` の有無と版を確かめる
- ADR-050を足し、SPEC.md、README、`docs/examples/screenshots.md` を更新する

## Capabilities

### New Capabilities
- `screenshots`: 画像の置き場所の取り決め、検査、issueへの添付

### Modified Capabilities
- `run-instructions`: 実装の実行だけの指示の種類
- `inbox-api`: 指示の種類に `implement` を足すこと
- `web`: 設定の画面に実装の実行だけの指示の節を足すこと

## Impact

- コード
  - `internal/instructions`：種類 `Implement`
  - `internal/agent`：指示の組み込み、置き場所の扱い、添付のコメント
  - `internal/prompt`：取り決めの節
  - `internal/workspace`：excludeと画像の集め方
  - `internal/forge`：`Attacher`（GitHubは `gh` の薄い層、GitLabはuploads API、fake）
  - `internal/marker`：添付のコメントの状態
  - `cmd/switchboard`：`instructions edit --implement`、`doctor`
  - `internal/inboxserve`、`inboxapi`、`web/`：指示の種類
- 文書：`docs/DECISIONS.md`（ADR-050）、`docs/SPEC.md`、`docs/ROADMAP.md`（V46、V47）を更新する。README、`docs/examples/screenshots.md`、`DESIGN.md` も合わせる
- 依存：GitHubのプロジェクトで画像を付けるには、端末に `gh`（v2.99.0以上）が要る。無くても実行と書き戻しは今までどおり動く
- 互換性：`implement.md` が無ければ今までと同じ指示になる。置き場所に画像が無ければ何もしない
