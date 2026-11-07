# OpenSpecを使うプロジェクトでの設定の例

OpenSpecで変更を管理しているプロジェクトでは、自動の実行にも、変更（change）を記録させたい。switchboardはOpenSpecを知らないので、汎用の2つの仕組みを使う。1つは追加で許可するツール（`allow_tools_extra`）で、もう1つは実装の実行への追加の指示（`run.md`）である。switchboardがどちらも渡すのは、ファイルを書く実行だけで、計画の実行には渡さない。

## 許可

`openspec` のCLIを、OpenSpecを使うプロジェクトにだけ許す。スキルを使うなら、全体に `Skill` を足す。`allowed-tools` を持つスキルは、`Skill` を許可しないと呼べない（ROADMAPの検証項目V41）。

```yaml
agent:
  allow_tools_extra: [Skill]
projects:
  - ref: gh:YOUR_GITHUB_USER/your-repo
    allow_tools_extra: ["Bash(openspec *)"]
```

## 指示

次の内容を、`switchboard instructions edit --run gh:YOUR_GITHUB_USER/your-repo` で書く。全プロジェクトに渡すなら、プロジェクトを付けずに書く。

```markdown
This repository plans every change as an OpenSpec change under openspec/changes/.

- Before changing code, create the change with `openspec new change <name>` (a kebab-case name for the issue) and
  commit it as the first commit of this run. Skip this when the branch already has a change for this issue.
- When the issue has an approved plan, record the questions asked in the issue and the answers to them in the
  change's grilling record, then write the proposal, specs, design and tasks from the approved plan. Do not reopen
  decisions the issue already settled.
- When the issue has a comment that skipped the questions and the approval, record in the grilling record why the
  questions were skipped and the plan of that comment, then write the other artifacts from that plan.
- When the issue has no plan, record in the grilling record that the interview was skipped because the run started
  without a plan stage, and write the other artifacts from the issue.
- Use `openspec instructions <artifact> --change <name> --json` for each artifact's template and rules.
- Tick each task in tasks.md as you finish it, and run `openspec validate <name> --strict` before you end the run.
- Do not use skills that interview a person or wait for an answer: nobody can answer during this run.
```

## 確かめ方

計画から始めたissueを1つ承認まで進める。承認のあとの実装の最初のコミットで、`openspec/changes/` に変更ができていればよい。`plan_first: auto` のプロジェクトで質問と承認を省いたissueでは、grillingの記録に、質問を省いた理由と省いたコメントの計画が書かれていればよい。実行の `stream.jsonl` の結果の `permission_denials` に、`Skill` と `openspec` の呼び出しが残っていないことも確かめる。
