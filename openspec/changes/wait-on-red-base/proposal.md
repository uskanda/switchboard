# Proposal

## Why

2026-11-15に、`gh:uskanda/smarternews` のmainへ、同じ番号のマイグレーションを足すPRが続けてマージされ、mainのCIが赤になった。
衝突を直す実行は、赤いmainを取り込んだ。そのあとのCIはmainと同じテストで落ち、エージェントはPRの失敗とみなして直す回数を使い切った。開いている10件のPRが `waiting(reason=ci)` で止まった。
PRの側では直せない失敗に、直す実行と回数を使っている。mainが直っても、PRは人が数え直すまで動かない。

## What Changes

- PRのCIが失敗したとき、エージェントはbaseのブランチの先頭のコミットのCIを読む。PRで失敗したチェックの名前がすべてbaseの先頭でも失敗していれば、「baseが赤」と判定する。baseのCIが終わっていなければ、判定を次の問い合わせに回す。
- baseが赤のとき、エージェントは直す実行を始めず、直した回数にも数えない。issueは `agent:done` のまま、見張りを続ける。待ち始めたときに、issueへ短いコメントを1回だけ書く。コメントには状態を変えない一時的なマーカー（`state=note reason=base`）を付ける。
- baseが赤の間は、衝突を直す実行も始めない。
- 待っている間、エージェントは `poll_interval` ごとに、プロジェクトとbaseの組ごとに1回だけbaseの先頭のCIを読む。待つ期間に上限は設けない。
- baseが緑に戻ったら、エージェントはClaudeを起動せずに、worktreeで `git merge origin/<base>` を実行してpushする。`fix=base` のマーカー付きのdoneの投稿を出し直し、新しいコミットのCIを見張る。取り込みで衝突したら、衝突を直す実行（回数に数える）に回す。
- 受信箱のマージの確認ダイアログ（ADR-050）に、baseのCIが失敗していることを出す。表示は情報だけで、マージは止めない。
- 決定をADR-075に、baseとPRでチェックの名前が揃うことの確認をROADMAPの検証項目V67に記録する。

受信箱からのマージの直列化（順番待ち、baseの取り込み、赤い間の保留）は、別の変更 `serialize-inbox-merges` で扱う。決定は `grilling.md` に記録してある。

## Capabilities

### New Capabilities

なし。

### Modified Capabilities

- `ci-watch`: 直す実行と衝突を直す実行の開始に、baseが赤でないことを条件に足す。baseが赤の判定、待ち、baseの読み取りの間隔、緑に戻ったときの取り込みの要件を足す。
- `inbox-api`: マージの前の状態に、baseのCIの失敗を足す。
- `web`: マージの確認ダイアログに、baseのCIの失敗を出す。

## Impact

- `internal/agent/ciwatch.go`（`checkWatch`、`ciFailed`、`conflicted`、`startFixes`）と、取り込みのための新しい処理。
- `internal/forge`：ブランチの先頭のコミットを読む口を `CI` に足し、GitHubとGitLabのアダプタで実装する。
- `internal/workspace`：baseを機械的にmergeする処理。
- `internal/marker`：一時的な状態 `note` と、`fix=base`。
- `internal/inboxserve/merge_status.go`、`inboxapi` の `MergeStatus`、Web表示のマージのダイアログ（日本語と英語の文言）。
- `docs/DECISIONS.md`（ADR-075）、`docs/ROADMAP.md`（V67）、`docs/DESIGN.md` のCIの見張りの節。
- トークンの権限と設定の項目は変わらない。baseのCIは、今の `Actions: Read` と `Commit statuses: Read`（GitLabは `api`）で読める。
