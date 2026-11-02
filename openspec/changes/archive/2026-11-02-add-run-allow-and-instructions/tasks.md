# Tasks

## 1. 検証項目

- [x] 1.1 `docs/ROADMAP.md` の検証項目にV41を足す。`--permission-mode dontAsk` の実物の `claude -p` で、allowへ `Skill` を入れたとき、Skillツールを呼べるかを確かめる。入れないとき、`permission_denials` へ記録されることも確かめる。結果の行が表に入っていることで確かめる
- [x] 1.2 V42を足す。allowの `Bash(git *)` と、denyの `Bash(git push *)` が重なるとき、`git push` が拒否されるかを確かめる。結果の行で確かめる

## 2. 設定

- [x] 2.1 `internal/config` に `<agent.allow_tools_extra>` と `<projects[].allow_tools_extra>` を足す。spec `config` の「項目が無い設定」のテストで確かめる
- [x] 2.2 `internal/config` に追加の許可の検査を書く（空、制限の無いBash、`git push`、`git rebase`、`gh`、`WebFetch`）。spec `config` の2つの誤りのシナリオと、`Bash(git *)` が通ることをテストで確かめる
- [x] 2.3 `<projects[].allow_tools_extra>` を設定の読み直しで反映し、`<agent.allow_tools_extra>` は再起動まで反映しないことをテストで確かめる。READMEの設定の例に2つの項目を書き足す

## 3. 実行の許可

- [x] 3.1 `settings` に追加の許可を渡し、全体、プロジェクトの順に重複なくつなげる。直接の実装、承認のあとの実装、直す実行の3つで `settings.json` に入ることをテストで確かめる
- [x] 3.2 `planSettings` が作る計画の実行の `settings.json` に追加の許可が入らないことと、denyの一覧が変わらないことをテストで確かめる

## 4. 実装用の指示

- [x] 4.1 `internal/instructions` に種別（`plan`、`run`）を足し、`Path`、`Read`、`Save` が種別を受け取るようにする。spec `run-instructions` の置き場所のシナリオと、トークンを拒むシナリオのテストで確かめる。監査ログに `kind` が入ることも確かめる
- [x] 4.2 `internal/prompt` の利用者の追加の指示の節を共通の関数にする。`System`、`ApprovedSystem`、`FixSystem` が実装用の指示を受け取って節を足すことをテストで確かめる。指示が空のときに節が無いことも確かめる
- [x] 4.3 `internal/agent` で、ファイルを書く実行のたびに実装用の指示を読み直して渡す。計画の実行には渡さないことと、動作中に書き換えた指示が次の実行に入ることをテストで確かめる
- [x] 4.4 `switchboard instructions edit` に `--run` を足す。`--run` ありで `<run.md>`、なしで `plan.md` が開くことをテストで確かめる。READMEの説明を書き足す

## 5. 受信箱のAPIとWeb表示

- [x] 5.1 `inboxapi` の `Instruction` と `SaveInstructionsRequest` に `kind` を足す。`switchboard inbox serve` のGETが対象ごとに2項目を返すことをテストで確かめる。PUTの `kind` が空なら計画用、未知なら誤りになることも確かめる
- [x] 5.2 `web/` の設定の画面に、対象ごとの「計画の指示」と「実装の指示」の欄を並べる。実装用の欄の保存で計画用の欄が変わらないことと、誤りが欄ごとに出ることをテストで確かめる。`DESIGN.md` の設定の画面の記述を合わせる

## 6. 文書

- [x] 6.1 `docs/DECISIONS.md` にADR-045を足す。追加の許可と、実装する実行への追加の指示の決定を書く。ADR-038の追加の指示は計画用だと補う。grillingの記録として `add-run-allow-and-instructions` の第1〜3ラウンドを示す
- [x] 6.2 `docs/SECURITY.md` のT7に、追加の許可は設定ファイルにしか書けないこと、読み込みのときの検査、denyは境界ではないことを足す
- [x] 6.3 `docs/` に、OpenSpecを使うプロジェクト向けの記入例を置く（`<allow_tools_extra>` と `<run.md>`）。製品のコード、既定の設定、記入例に `uskn` が無いことを `grep` で確かめる（確かめる既存のテストは無かった）
- [x] 6.4 `make verify` が通ることを確かめる

## 7. 開発者の端末（手作業）

- [x] 7.1 `switchboard` をこの変更を入れた版にして、常駐のサービスを入れ直す。設定に `<agent.allow_tools_extra>`（`Skill`）を書く。OpenSpecを使うプロジェクトには `<projects[].allow_tools_extra>`（`Bash(openspec *)`）を書く
- [x] 7.2 6.3の記入例をもとに、`<instructions/run.md>` を `switchboard instructions edit --run` で書く
- [x] 7.3 `agent:plan` のissueを1つ承認まで進める。実装の実行の最初のコミットで `openspec/changes/` にchangeができることを確かめる。`stream.jsonl` で、Skillと `openspec` の呼び出しが拒否されていないことを確かめる
