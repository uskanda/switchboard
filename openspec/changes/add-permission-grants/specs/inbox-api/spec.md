## MODIFIED Requirements

### Requirement: 追加の指示の保存
APIは、指定した対象と種別の指示を保存しなければならない（MUST）。保存の前の検査と記録は、`plan-instructions` の保存と同じにする。
種別を省略した保存は、計画用の指示の保存として扱わなければならない（MUST）。
検査で拒んだとき、APIは保存せずに、大きさの上限を超えたことか、秘密らしいものがあることを区別できる誤りを返さなければならない（MUST）。
設定に無いプロジェクトか、未知の種別を指定したとき、APIは保存せずに誤りを返さなければならない（MUST）。
APIは、指示の保存の口で、許可の規則を読み書きしてはならない（MUST NOT）。許可の規則は、許可の口だけで扱う。

#### Scenario: 大きすぎる指示
- **WHEN** 40 KiBの指示を保存しようとする
- **THEN** 保存されず、大きさの上限を超えたことを示す誤りが返る

#### Scenario: 設定に無いプロジェクト
- **WHEN** 設定に無い `gh:someone/other` の指示を保存しようとする
- **THEN** 保存されず、誤りが返る

#### Scenario: 種別を省略した保存
- **WHEN** 種別を付けずに、`gh:uskanda/switchboard` の指示を保存する
- **THEN** `<instructions/gh/uskanda/switchboard/plan.md>` が書き換わり、実装用の `<run.md>` は変わらない

## ADDED Requirements

### Requirement: 許可の読み出し
APIは、全体の許可と、設定にあるプロジェクトごとの許可を返さなければならない（MUST）。issueを指定したときは、そのissueの許可も返す。
各項目は、範囲（全体、`gh:owner/repo` の形のプロジェクト、issueの参照）、規則の一覧、`switchboard inbox serve` が動く端末の名前を持つ。ファイルが無い範囲は、空の一覧として返す。
`<inbox.auth>` のある構成では、APIは、管理者でない人に許可を返してはならない（MUST NOT）。

#### Scenario: プロジェクトの許可だけがある
- **WHEN** `gh:uskanda/smarternews` の許可に `Bash(git mv *)` だけがある状態で、許可を求める
- **THEN** 全体の一覧は空で、`gh:uskanda/smarternews` の一覧に `Bash(git mv *)` が入って返る

### Requirement: 許可の追加
APIは、範囲と規則の一覧を受けて、その範囲の許可のファイルに規則を足さなければならない（MUST）。すでにある規則は足さない。
規則の検査は `permission-grants` の「Web表示から足す規則の検査」に従う。1つでも断られたとき、APIは何も保存せずに、断った規則と理由を返さなければならない（MUST）。
設定に無いプロジェクトか、存在しないissueを指定したとき、APIは保存せずに誤りを返さなければならない（MUST）。
`<inbox.auth>` のある構成では、APIは、管理者でない人の追加を403で断らなければならない（MUST）。
保存したとき、APIは、監査ログに `web.action` と `<permission.saved>`（操作した人、範囲、足した規則）を残さなければならない（MUST）。

#### Scenario: issueに足す
- **WHEN** `gh:uskanda/smarternews#217` に、範囲をissueとして `Bash(git ls-tree *)` を足す
- **THEN** `<issues/217.txt>` に規則が書かれ、issueにはコメントが投稿されない

#### Scenario: 断られる規則が混じる
- **WHEN** `Bash(git mv *)` と `Bash(git *)` を一緒にプロジェクトの許可に足そうとする
- **THEN** どちらも保存されず、`Bash(git *)` を断った理由が返る

#### Scenario: 管理者でない人
- **WHEN** `<inbox.auth>` のある構成で、管理者でない人が許可を足そうとする
- **THEN** 403が返り、ファイルは変わらない

### Requirement: 許可の削除
APIは、範囲と規則を受けて、その範囲の許可のファイルから規則を外さなければならない（MUST）。ファイルにない規則を指定したときは、何もせずに成功を返す。
権限と監査ログは、許可の追加と同じにする。監査ログの出来事は `<permission.removed>` とする。

#### Scenario: issueの許可を外す
- **WHEN** issue 217の許可から `Bash(git ls-tree *)` を外す
- **THEN** `<issues/217.txt>` からその行が消え、監査ログに `<permission.removed>` が残る

### Requirement: 拒否された操作の読み出し
1件の詳細を返すとき、issueがpermissionの待ちなら、APIは、拒否された操作の一覧を返さなければならない（MUST）。
一覧は、エージェントの最新の `waiting(reason=permission)` のコメントの、拒否された操作の節から読む。APIは、投稿者がエージェントであることを確かめなければならない（MUST）。
各項目は、ツールの名前、コマンド、`permission-grants` で作った規則の候補を持つ。候補を作れないコマンドは、候補を空にし、つないだコマンドであることを示す。
APIは、拒否された操作を、フォージから読んだデータとして扱わなければならない（MUST）。

#### Scenario: gitのサブコマンドの拒否
- **WHEN** 最新のpermissionの待ちのコメントに `- Bash: \`git mv a.sql b.sql\`` がある
- **THEN** 詳細の拒否の一覧に、コマンド `git mv a.sql b.sql` と、候補 `Bash(git mv)` と `Bash(git mv *)` が入る

#### Scenario: 人が書いた似たコメント
- **WHEN** 信頼済みの人が、拒否の節と同じ形の行を含むコメントを書いた
- **THEN** その行は拒否の一覧に入らない
