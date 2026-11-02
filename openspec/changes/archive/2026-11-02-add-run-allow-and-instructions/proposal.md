# Proposal

## Why

switchboardの自動実行では、手作業なら守れる開発の規約を守れないことがある。たとえば、OpenSpecを使うプロジェクトでもchangeを作らずに実装する。利用者の `~/.claude/CLAUDE.md` は実行に届いている。しかし、実装する実行にはSkillツールや `openspec` のCLIの許可が無く、`dontAsk` で拒否される。さらに、実装する実行のsystem promptに利用者が規則を足す手段も無い。実装する実行68件のうち、`openspec/changes/` に書いたのは13件だけだった。

## What Changes

- 設定に、追加で許可するツールの一覧 `<allow_tools_extra>` を足す。全体の `<agent.allow_tools_extra>` と、プロジェクトごとの `<projects[].allow_tools_extra>` に置け、両方をつなげて使う
- 設定の読み込みのときに、追加の許可を検査する。組み込みのdenyに当たる項目と、無制限の `Bash`、`Bash(*)` は誤りとして起動を止める
- 追加の許可は、ファイルを書く実行のすべて（直接の実装、承認のあとの実装、CIとpushの失敗を直す実行、baseとの衝突を直す実行）に足す。計画の実行には足さない
- 計画用の `plan.md` とは別の、実装する実行のための利用者の追加の指示 `<run.md>` を足す。置き場所は全体とプロジェクトごとの2つで、内容は、ファイルを書く実行のsystem promptへ入れる
- `<run.md>` の保存の検査、監査ログ、CLIでの編集、受信箱のAPIとWeb表示での編集は、`plan.md` と同じにする。Web表示とAPIでは、追加の許可は編集できない
- `docs/` に、OpenSpecを使うプロジェクト向けの `<run.md>` と `<allow_tools_extra>` の記入例を載せる。製品のコードと既定の設定では、OpenSpecとハーネスを扱わない（ADR-021）
- ADR-045を足し、SECURITY.mdのT7を書き換える

## Capabilities

### New Capabilities
- `run-instructions`: 実装する実行に渡す利用者の追加の指示（`<run.md>`）の置き場所、読み込み、渡し方、保存、編集

### Modified Capabilities
- `config`: 追加で許可するツールの設定と、その検査
- `agent-run`: ファイルを書く実行の許可に、追加の許可を足すこと
- `inbox-api`: 追加の指示の読み出しと保存で、計画用と実装用を区別すること
- `web`: 設定の画面で、計画用と実装用の指示を編集できること

## Impact

- コード
  - `internal/config`：設定の項目と検査
  - `internal/agent`：許可の組み立て、指示の読み込み
  - `internal/prompt`：実装する実行のsystem promptの節
  - `internal/instructions`：`<run.md>` の置き場所
  - `cmd/switchboard`：`instructions edit` のフラグ
  - `internal/inboxserve` と `inboxapi`：APIの種別
  - `web/`：設定の画面
- 文書：`docs/DECISIONS.md`（ADR-045）、`docs/SECURITY.md`（T7）、README、OpenSpecを使うプロジェクト向けの記入例
- 互換性：新しい設定の項目は省略でき、省略したときの動きは今と同じ。APIは種別を省略すると計画用として扱う
- 開発者の端末：`<instructions/run.md>` と設定の `<allow_tools_extra>` を手で書く（tasksの最後の手順）
