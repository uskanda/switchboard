# Spec Delta

## ADDED Requirements

### Requirement: コメントとノートを全件読んで編集者を確かめる
信頼の判定は、issueのコメントやノートが何件あっても、すべての編集者を確かめなければならない（MUST）。

#### Scenario: 101件目のコメント
- **WHEN** GitHubのissueに101件以上のコメントがあり、最新のコメントが信頼済みの人の返答である
- **THEN** 判定はその返答の編集者を読み、返答として信頼する

### Requirement: タイトルの編集者を確かめる
issueのタイトルを信頼されない人が変えたとき、エージェントはそのissueに着手してはならない（MUST NOT）。

#### Scenario: 信頼されない人のタイトルの変更
- **WHEN** 信頼済みの人が作ったissueのタイトルを、信頼されない人が変える
- **THEN** エージェントは着手せず、理由を監査ログに残す

### Requirement: GitLabの本文の編集者は本文の変更のノートで決める
GitLabで、判定は、本文を変えたことを示すシステムノートだけで本文の編集者を決めなければならない（MUST）。

#### Scenario: 件名に「description」を含める
- **WHEN** 信頼されない人が、件名に「description」を含むタイトルへ変える
- **THEN** 本文の編集者は変わらない
