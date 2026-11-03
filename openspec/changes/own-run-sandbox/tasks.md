# Tasks

## 1. 実物での確認

- [x] 1.1 `excludedCommands` に `make verify` を入れたとき、`make deploy` はsandboxの中で動くことを、実物の `claude` で確かめる
- [x] 1.2 `denyRead` に入れたパスが無いとき、sandboxがそのパスにファイルを置くかを確かめる
- [x] 1.3 `docs/ROADMAP.md` の検証項目に、確認の結果と、grillingの間に確かめた4項目を書く

## 2. 起動の引数と設定ファイル

- [x] 2.1 `claudecode` の `Args` に `--setting-sources project,local` を足し、`claudecode_test.go` で確かめる
- [x] 2.2 `claudecode.Settings` に、sandboxの外で動かすコマンドと読めなくするパスを足す。`Run` が書く設定ファイルのsandboxの項目を、テストで確かめる
- [x] 2.3 sandboxを使えるかを確かめる関数を `claudecode` に足す。bwrapの成功と失敗を、偽のbwrapでテストする

## 3. 実行ごとのsandboxの中身

- [x] 3.1 `git` と検証コマンドの全体を、ファイルを書く実行ではsandboxの外に出す。計画の実行では `git` だけにする。両方をテストで確かめる
- [x] 3.2 `denyRead` に入れるパスの一覧を作る。資格情報の一覧、設定ファイルのディレクトリ、状態のディレクトリの `worktrees` 以外の項目を入れ、端末に無いパスを除く。テストで確かめる
- [x] 3.3 `Agent` に設定ファイルのパスを持たせ、`switchboard agent run` から渡す
- [x] 3.4 claudeの起動の前にsandboxを確かめ、失敗したら実行を誤りにして理由をコメントに書く。偽のbwrapでテストする

## 4. システムプロンプト

- [x] 4.1 `prompt.WithSandbox` でファイルを書く実行のシステムプロンプトにsandboxの節を足し、検証コマンドと `git` を単独で実行する指示を書く。テストで確かめる
- [x] 4.2 `prompt.PlanSystem` の「Tools」の節を、sandboxの中のコマンドが動くことに合わせて書き直す

## 5. 実物での確認

- [x] 5.1 `realclaude` のタグの付いたテストを足す。許可の規則の無い `python3` が動き、worktreeの外への書き込みと `gh` が拒否されることを確かめる

## 6. 文書

- [x] 6.1 `docs/DECISIONS.md` にADRを足す（sandboxの中身、自動の許可、sandboxの外のコマンド、読めなくするパス、bwrapの確認）
- [x] 6.2 `docs/SECURITY.md`、`docs/DESIGN.md`、`README.md` の許可とsandboxの説明を直す

## 7. 確認

- [x] 7.1 `make verify` が通ることを確かめる
- [x] 7.2 `openspec validate own-run-sandbox --strict` が通ることを確かめる
