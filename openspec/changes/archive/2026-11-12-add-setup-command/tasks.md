# Tasks

## 1. 設定

- [x] 1.1 `internal/config` の失敗するテストを書く。`<projects[].setup_command>` を読めること、項目が無い設定を読めること、`agent:` の下の `setup_command` が未知のキーになることを確かめる
- [x] 1.2 `config.Project` に `SetupCommand` を足し、1.1のテストが通ることを確かめる
- [x] 1.3 `projects` の読み直しのテストに、動いている間に足した `setup_command` が次の実行で使われる場合を足し、通ることを確かめる
- [x] 1.4 `config/` の設定の例に `setup_command` をコメントで足し、設定の例の検査（`make verify`）が通ることを確かめる

## 2. gitの状態の比較

- [x] 2.1 `internal/workspace` の失敗するテストを書く。無視の対象のファイルだけが増えた場合は同じと判定し、追跡しているファイルの書き換えと、無視の対象でない新しいファイルは違うと判定してパスを返すことを確かめる。前の実行の差分が残っていて変わらない場合と、その差分のあるファイルがさらに書き換わる場合も入れる
- [x] 2.2 `workspace.Space` に、gitの状態を取るメソッドと、2つの状態を比べて変わったパスを返す関数を足し、2.1のテストが通ることを確かめる

## 3. 準備のコマンドの実行

- [x] 3.1 `internal/agent` の失敗するテストを書く。準備のコマンドが `claude -p` の起動の前にworktreeで実行されること、2回目の実行と計画の実行でも実行されること、項目が無いプロジェクトでは実行されないことを確かめる
- [x] 3.2 `work` の失敗のテストを書く。場合は、0以外の終了、期限切れ、gitの状態の変化の3つとする。それぞれで、`claude -p` が起動せず、結果が `waiting(reason=error)` になることを確かめる。コメントに、コマンドと出力の末尾（または変わったパス）があることも確かめる。取り消しでは結果が `nil` になることも確かめる
- [x] 3.3 `work` の環境のテストを書く。準備のコマンドの環境変数にフォージのトークンが無いことを確かめる
- [x] 3.4 `work` に準備のコマンドの実行を足し、`messages.go` に失敗の文言を日本語と英語で足す。3.1から3.3のテストが通ることを確かめる
- [x] 3.5 `<setup.ran>` と `<setup.log>` のテストを書いてから実装する。監査ログに `<setup.ran>` が1行残り、出力を含まないこと、実行のディレクトリに `<setup.log>` が権限 `0600` で残ることを確かめる
- [x] 3.6 `settings` のテストに、`setup_command` のあるプロジェクトでも `excludedCommands` と許可の規則が変わらない場合を足し、通ることを確かめる

## 4. 追記する指示

- [x] 4.1 `internal/prompt` の失敗するテストを書く。準備のコマンドのあるときは節にコマンドの文字列があり、無いときは節が無いことを確かめる
- [x] 4.2 `internal/prompt` に準備のコマンドの節を足す関数を作り、`work` でsandboxの節の直後に足す。4.1のテストと、`--append-system-prompt-file` の中身を見る `internal/agent` のテストが通ることを確かめる

## 5. 文書

- [x] 5.1 `docs/DECISIONS.md` にADR-070を足し、`docs/DESIGN.md` の実行の節に準備のコマンドを書く。`make verify` の文書の検査が通ることを確かめる
- [x] 5.2 `docs/GUIDE.md` と `docs/GUIDE.ja.md` に、`setup_command` の説明を同じ内容で足す。勧めるコマンドの例、実行のたびに走ること、出力を `.gitignore` に入れること、失敗したときのコメントを書く。2つの版の見出しが対応していることを確かめる
- [x] 5.3 `docs/SECURITY.md` と `docs/SECURITY.ja.md` に、同じ内容の節を足す。書くのは、準備のコマンドがsandboxの外で動くこと、渡さない資格情報、依存の定義を変えるPRを人が確かめることである。2つの版の見出しが対応していることを確かめる
- [x] 5.4 `README.md` と `README.ja.md` の設定の説明に `setup_command` があるべきかを確かめ、プロジェクトの項目の一覧があれば同じ変更で足す

## 6. 全体の確認

- [x] 6.1 `make verify` をsandboxの外で実行し、通ることを確かめる
- [x] 6.2 `config.yaml` のsmarternewsのプロジェクトに `setup_command: pnpm install --frozen-lockfile` を足し、計画の実行を1回動かす。worktreeに `node_modules` ができ、実行の中で `pnpm exec vitest run` が通り、監査ログに `<setup.ran>` が残ることを確かめる。結果を `docs/ROADMAP.md` の検証項目に書く
