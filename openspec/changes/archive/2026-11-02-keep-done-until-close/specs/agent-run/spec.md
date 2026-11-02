## MODIFIED Requirements

### Requirement: 返答による再開
openのissueが `agent:waiting` か `agent:done` で、新しい信頼済みコメントがあるとき、エージェントは実行を始めなければならない（MUST）。始める前に、状態を `agent:running` に変える。
新しいコメントとは、エージェントの最新の投稿のマーカーの `ack` が指すコメントより後のコメントをいう。その投稿は、待ちかdoneの投稿でなければならない。着手、進捗、順番待ちのコメントは数えない。
GitLabでは、コメントでissueの更新時刻が変わらないので、エージェントは `agent:waiting` と `agent:done` のissueのコメント数の変化も見なければならない（MUST）。

#### Scenario: 返答で再開する
- **WHEN** 待ちのissueに、マスターが返答のコメントを書く
- **THEN** 次のポーリングでissueは `agent:running` になり、返答を含むプロンプトで `claude -p` が起動する

#### Scenario: 完了のissueへのコメントで再開する
- **WHEN** エージェントがdoneにしたopenのissueで、マスターが「レビューの指摘を直してください」とコメントする
- **THEN** 次のポーリングでissueは `agent:running` になり、そのコメントを含むプロンプトで `claude -p` が起動する

#### Scenario: GitLabの完了のissueへのコメント
- **WHEN** エージェントがdoneにしたGitLabのissueで、マスターがノートを書く
- **THEN** issueの更新時刻が変わらなくても、次のポーリングで実行が始まる

#### Scenario: 計画を承認したissueの再開
- **WHEN** 計画を先に立てるモードで、承認された計画を実装してdoneになったissueがある。そのissueでマスターがコメントする
- **THEN** 再開した実行は、承認された計画の実装として、計画と承認のコメントを名指すシステムプロンプトで動く

## REMOVED Requirements

### Requirement: 完了後のコメント
**Reason**: doneの後も、PR/MRの問題やコンフリクトのためにやりとりが要る（ADR-042）。
**Migration**: 要件の返答による再開に、`agent:done` のissueを含めた。人間がラベルを付けただけで、エージェントのdoneの投稿が無い `agent:done` のissueは、今までどおり再開しない。

## ADDED Requirements

### Requirement: 開いたPR/MRの使い続け
doneで終える実行は、pushの後に、issueのブランチを元とする開いたPR/MRを探さなければならない（MUST）。あれば、新しく作らずにそのURLでdoneにし、doneの投稿には「PRを更新しました」（GitLabでは「MRを更新しました」）とそのURLを書く。無ければ、今までどおりdraftのPR/MRを作る。
closeやmergeの済んだPR/MRは、使ってはならない（MUST NOT）。

#### Scenario: 再開した実行が再びdoneになる
- **WHEN** doneのissueへのコメントで再開した実行が、commitしてdoneを宣言する
- **THEN** PRは1つのままで、doneの投稿に「PRを更新しました」とそのPRのURLが出る

#### Scenario: PRがcloseされていた
- **WHEN** 前のPRがcloseされた後に、再開した実行がdoneを宣言する
- **THEN** 新しいdraftのPRが作られる

### Requirement: baseとの衝突の解消
実装する実行のシステムプロンプトは、baseのブランチとの衝突を `git merge origin/<base>` で解消するよう指示しなければならない（MUST）。rebase、amend、履歴の書き換えはしないよう、あわせて指示する。実装する実行の許可ツールは、`git merge` を含まなければならない（MUST）。
エージェントのpushは、forceを使ってはならない（MUST NOT）。

#### Scenario: システムプロンプトの規則
- **WHEN** baseのブランチが `develop` のプロジェクトで、実装する実行が始まる
- **THEN** システムプロンプトに `git merge origin/develop` で解消する規則と、rebaseをしない規則が入る
