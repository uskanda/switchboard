# Tasks

## 1. GitHubのエラーの詳細

- [x] 1.1 `internal/forge/github` で、失敗の応答の `errors` をエラー文に含める。`errors` 付きの422を返すテストサーバーで、エラー文に各項目の `message` が入ることを `go test ./internal/forge/github/` で確かめる

## 2. 開いた変更要求を探す操作

- [x] 2.1 `forge.Forge` に `OpenChangeRequest` を足し、fakeに実装する。fakeの変更要求に開閉の状態を持たせ、閉じたものは返さないことを `go test ./internal/forge/fake/` で確かめる
- [x] 2.2 GitHubの実装を書く。`state=open`、`head=<owner>:<branch>`、`base` を送ることを確かめる。空の配列で `ErrNotFound` を返すことも、`go test ./internal/forge/github/` で確かめる
- [x] 2.3 GitLabの実装を書く。`state=opened`、`source_branch`、`target_branch` を送ることを確かめる。空の配列で `ErrNotFound` を返すことも、`go test ./internal/forge/gitlab/` で確かめる

## 3. 完了での再利用

- [x] 3.1 `finish` で、作る前に開いた変更要求を探し、あればそのURLで完了にする。開いたPRがあるときに新しいPRが作られず、完了のコメントに既存のURLが書かれることと、閉じたPRだけのときに新しく作られることを、`internal/agent` のテストで確かめる
- [x] 3.2 `make verify` が通ることを確かめる
