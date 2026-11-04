# Spec Delta

## ADDED Requirements

### Requirement: 外部コマンドを時間切れと取り消しで止める
`verify_command` か `check_command` が時間切れになるか取り消されたとき、エージェントは、そのコマンドのプロセスグループ全体を止め、出力のパイプを開いたままの孫プロセスが残っても待ち続けずに戻らなければならない（MUST）。

#### Scenario: パイプを開いたままの孫プロセス
- **WHEN** `verify_command` が、出力のパイプを開いたまま眠り続ける孫プロセスを起動し、実行の時間切れが来る
- **THEN** エージェントはコマンドの終わりを待ち続けず、時間切れとして戻る

### Requirement: ADR-018の検査はissueのブランチの分岐点と比べる
エージェントは、作業ツリーの `CLAUDE.md`、`AGENTS.md`、`.claude/`、`.mcp.json` を、`origin/<base>` の最新ではなく、issueのブランチと `origin/<base>` の分岐点と比べなければならない（MUST）。

#### Scenario: worktreeを作った後にベースのCLAUDE.mdが変わる
- **WHEN** worktreeを作った後に、ベースブランチのCLAUDE.mdが更新される
- **THEN** そのissueは検査を通り、verify待ちにならない

#### Scenario: issueのブランチでの変更
- **WHEN** issueのブランチのコミットか未追跡のファイルが、CLAUDE.mdを変える
- **THEN** 検査は今までどおりその変更を検出する

### Requirement: 下の階層の指示ファイルも検査する
エージェントは、どの階層にある `CLAUDE.md`、`AGENTS.md`、`.claude/` も、ADR-018の検査とpush前の保護パスの検査の対象にしなければならない（MUST）。

#### Scenario: サブディレクトリのCLAUDE.md
- **WHEN** 実行が `sub/CLAUDE.md` を足す
- **THEN** push前の検査はその変更を拒む

### Requirement: 保護パスからのリネームを拒む
保護パスのファイルが保護パスの外へリネームされたとき、エージェントはpushしてはならない（MUST NOT）。

#### Scenario: CLAUDE.mdを外へ移す
- **WHEN** 実行が `CLAUDE.md` を `notes.md` にリネームしてコミットする
- **THEN** push前の検査は、元のパスの変更として拒む

### Requirement: 他の端末が実行中のissueを回復処理が書き戻さない
エージェントの起動時の回復処理は、最新の着手のコメントのhostが自分でないissueを、中断として書き戻してはならない（MUST NOT）。

#### Scenario: 他の端末の実行
- **WHEN** 端末Aが実行中のissueを、端末Bのエージェントが起動時に見つける
- **THEN** 端末Bはそのissueに何も書き戻さない

### Requirement: 2台の着手の競合で勝つのは1台だけ
2台のエージェントが同じissueに着手のコメントを投稿したとき、着手の判定は、どちらの端末から見ても同じ1台だけを勝ちとしなければならない（MUST）。

#### Scenario: 相手の着手を読んだ後の判定
- **WHEN** 端末Bが、端末Aの着手のコメントを読んだ後に自分の着手を判定する
- **THEN** 両方の端末が、同じ1台を勝ちと判断する

### Requirement: 1つのプロジェクトの失敗で起動を止めない
起動時に1つのプロジェクトで公開か非公開かを読めないとき、エージェントは、ほかのプロジェクトで起動し、そのプロジェクトを次のポーリングでやり直さなければならない（MUST）。

#### Scenario: RepoPrivateの失敗
- **WHEN** 起動時に、あるプロジェクトの `RepoPrivate` がエラーを返す
- **THEN** エージェントは起動し、ほかのプロジェクトのissueを処理する

### Requirement: 古い実行ディレクトリを消す
`agent.retention` が0でない間、エージェントは、起動時と1日ごとに、最後の書き込みから保持期間を過ぎた実行ディレクトリを消さなければならない（MUST）。実行中のもの、書き戻しの待ちのもの、各issueの最新の記録が指すものは残さなければならない（MUST）。

#### Scenario: 保持期間を過ぎた実行
- **WHEN** 31日前に終わった実行のディレクトリがあり、`agent.retention` が既定の720hである
- **THEN** エージェントは起動時にそのディレクトリを消す

#### Scenario: issueの最新の実行
- **WHEN** 保持期間を過ぎた実行が、そのissueの最新の記録に指されている
- **THEN** エージェントはそのディレクトリを残す
