# Proposal

## Why

switchboardはまだ設計文書だけで、動くものが無い。ROADMAP.mdのPhase 1の完了条件は、issueに `agent:ready` を付けると、待ち、返答、PRの作成までが手でコマンドを打たずに回ることである。前提にするClaude Codeの挙動はPhase 0で確かめ、設計の判断はgrilling（`grilling.md`）とADRで決めた。そのため、最小構成を実装に移せる。

## What Changes

- Goで書いた `switchboard` コマンドを新しく作る。Phase 1のサブコマンドは `labels sync`、`agent run`、`agent status`、`inbox`、`show`、`reply`、`doctor` の7つとする
- 対象はGitHubだけとし、更新はポーリングで検知する。対象のリポジトリは1つ、端末はUbuntuの1台を想定する
- 設定ファイルを読み込み、トークンはSecret Serviceから取り出す。開発時に限り、環境変数からも読める
- 状態ラベル（`agent:ready`・`agent:running`・`agent:waiting`・`agent:done`）を作り、issueに常に1つだけ付ける
- issueの本文、コメント、ラベルの付与者、編集者を、ユーザーIDで信頼判定する
- 信頼できる入力だけからプロンプトを作る。`claude -p` は、許可リストから組み立てた環境変数を渡して起動する。Claudeのプロセスにはフォージのトークンを渡さない
- 実行は構造化出力で `waiting` か `done` を宣言して終える。エージェントが結果を判定し、コメント、ラベル、push、draft PRを書き戻す
- 受信箱は、設定済みのプロジェクトから待ちのissueを集めて表示し、そこから返答を投稿できる
- ライセンスファイル（MIT）と、ハーネスを入れないCIを追加する

## Capabilities

### New Capabilities

- `config`: 設定ファイルの読み込みと検証、資格情報の取り出し
- `state-labels`: 状態ラベルの作成と、issueごとに1つを保つ付け替え
- `trust`: 指示として採用する入力と、エージェントの投稿を見分ける信頼判定
- `agent-run`: issueの検知から `claude -p` の起動、停止、再起動後の回復まで
- `run-outcome`: 実行結果の判定と、フォージへの書き戻し（コメント、ラベル、push、PR）
- `inbox`: 待ちのissueの一覧と、そこからの返答

### Modified Capabilities

なし。

## Impact

- 新しいGoモジュールとソースコード（`cmd/`、`internal/`）を追加する。外部依存は、GitHub APIのクライアント、YAMLの読み込み、Secret Serviceへの接続に限る
- 実行環境には、`git`、`claude`（2.1.312で確認済み）、D-Bus経由のSecret Serviceが要る
- `config/switchboard.example.yaml`、README.md、`Makefile` の `verify` を、実装に合わせて更新する
- DECISIONS.mdに、第6ラウンドの決定をADRとして追加する
