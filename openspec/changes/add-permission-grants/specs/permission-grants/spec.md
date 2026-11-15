## Purpose

利用者がWeb表示から足す実行の許可の規則を、全体、プロジェクト、issueの3つの単位でファイルに置き、ファイルを書く実行に渡す。足せる規則の検査と、拒否された操作から規則の候補を作る決まりも持つ。

## ADDED Requirements

### Requirement: 許可のファイルの置き場所
許可の規則は、設定ファイルと同じディレクトリの `<permissions/>` の下に置かなければならない（MUST）。
全体の許可は `<permissions/allow.txt>` とする。
プロジェクトの許可は `<permissions/<gh|gl>/<owner>/<repo>/allow.txt>` とする。GitLabのグループが入れ子なら、各段をディレクトリにする。
issueの許可は、そのプロジェクトのディレクトリの `<issues/<番号>.txt>` とする。
ファイルは0600、ディレクトリは0700で作らなければならない（MUST）。

#### Scenario: issueの許可を保存する
- **WHEN** `gh:uskanda/smarternews` のissue 217に `Bash(git ls-tree *)` を足す
- **THEN** 設定ファイルと同じディレクトリの `<permissions/gh/uskanda/smarternews/issues/217.txt>` に、0600で1行が書かれる

### Requirement: 許可のファイルの形
許可のファイルは、1行に1つの規則を書くテキストでなければならない（MUST）。
読むとき、空白だけの行と、`#` で始まる行は無視しなければならない（MUST）。行の前後の空白は取り除く。
同じ規則が2行あるとき、1つとして扱わなければならない（MUST）。

#### Scenario: 手で書いたファイル
- **WHEN** `<permissions/allow.txt>` に、コメントの行と空行と `Skill` の行がある
- **THEN** 全体の許可は `Skill` の1つとして読まれる

### Requirement: 許可のファイルの読み込み
ファイルを書く実行を始めるたびに、エージェントは、3つの単位の許可のファイルを読み直さなければならない（MUST）。3つとは、全体、そのプロジェクト、そのissueである。
読んだ規則は、設定ファイルの2つの `allow_tools_extra` のあとに、全体、プロジェクト、issueの順でつなげなければならない（MUST）。
読んだ規則のうち、Web表示からの検査に通らないものは、エージェントは使ってはならず、ログに1回残さなければならない（MUST）。
ファイルが無いときは、その単位の許可を無いものとして扱わなければならない（MUST）。
エージェントは、計画の実行に、許可のファイルの規則を渡してはならない（MUST NOT）。

#### Scenario: 動いている間に許可を足す
- **WHEN** エージェントが動いている間に、Web表示からプロジェクトの許可に `Bash(git ls-tree *)` を足し、そのあとで実装の実行が始まる
- **THEN** その実行の許可に `Bash(git ls-tree *)` が入る

#### Scenario: 別のissueの許可
- **WHEN** issue 217の許可に `Bash(git mv *)` があり、同じプロジェクトのissue 218の実装の実行が `git mv` を実行しようとする
- **THEN** コマンドは拒否され、resultの `permission_denials` に記録される

#### Scenario: 手で書いた危ない規則
- **WHEN** `<permissions/allow.txt>` に手で `Bash(git config *)` を書き、実装の実行が始まる
- **THEN** その規則は実行に渡されず、ログに残る

#### Scenario: 計画の実行
- **WHEN** 全体の許可に `Skill` がある状態で、計画の実行が始まる
- **THEN** 計画の実行の許可に `Skill` は入らない

### Requirement: Web表示から足す規則の検査
Web表示と受信箱のAPIから足す規則には、`<agent.allow_tools_extra>` と同じ検査をかけなければならない（MUST）。空の規則、制限の無い `Bash`、拒否している操作（`git push`、`git rebase`、`gh`、`WebFetch`）を名指す規則を断る。
そのうえで、次の規則も断らなければならない（MUST）。
- `git config` と `git -c` を名指す規則
- サブコマンドを限らないgitの規則（`Bash(git *)`、`Bash(git:*)` など）
- ワイルドカードだけの規則（`Bash(*)` など）
規則の長さは200文字までとする。超える規則は断らなければならない（MUST）。

#### Scenario: サブコマンドを限らないgit
- **WHEN** Web表示から `Bash(git *)` を全体の許可に足そうとする
- **THEN** 保存されず、断った理由が返る

#### Scenario: 設定ファイルには書ける
- **WHEN** `config.yaml` の `<agent.allow_tools_extra>` に `Bash(git *)` を書く
- **THEN** 今までどおり設定の誤りにはならない

### Requirement: 拒否された操作からの候補
拒否された操作が1つのBashのコマンドのとき、規則の候補は、先頭の語とサブコマンドから作らなければならない（MUST）。gitなら、`git ls-tree --name-only origin/main` から `Bash(git ls-tree)` と `Bash(git ls-tree *)` を作る。gitでないコマンドは、先頭の語から作る（`mkdir -p x` なら `Bash(mkdir)` と `Bash(mkdir *)`）。
パイプ、`&&`、`;`、`for` などでつないだコマンドには、候補を作ってはならない（MUST NOT）。
Bashでないツールの拒否には、ツールの名前を候補にしなければならない（MUST）。
Web表示から足す規則の検査に通らない候補は、作ってはならない（MUST NOT）。

#### Scenario: gitのサブコマンド
- **WHEN** `git mv packages/a.sql packages/b.sql` が拒否された
- **THEN** 候補は `Bash(git mv)` と `Bash(git mv *)` である

#### Scenario: つないだコマンド
- **WHEN** `for f in a b; do git log -- $f; done` が拒否された
- **THEN** 候補は無く、規則では許可できないことが示される

#### Scenario: 危ないgit
- **WHEN** `git -c core.pager=sh log` が拒否された
- **THEN** 候補は無い
