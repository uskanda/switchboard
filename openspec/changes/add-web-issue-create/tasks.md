# Tasks

## 1. 決定の記録

- [x] 1.1 `docs/DECISIONS.md` にADR-040を足す。`docs/SECURITY.md` のT5に、issueの作成のPOSTも同じ検査をすることを足す
- [x] 1.2 `docs/SPEC.md` のF4、`DESIGN.md` の部品、README.mdのWeb表示の節に、issueの作成を足す

## 2. フォージ（`forge`）

- [x] 2.1 `forge.Issues` を足し、GitHub、GitLab、偽のフォージで実装する。`httptest` のサーバーで、要求のパス、タイトル、本文、ラベル（GitLabはカンマ区切り）と、無いリポジトリで `ErrNotFound` になることをテストする。偽のフォージでは、作った人がラベルを付けた人として残ることを確かめる

## 3. 受信箱のサービス（`inboxapi`、`inboxserve`、`cmd/switchboard`）

- [x] 3.1 `inboxapi` に、作れる先の一覧と作成の型、クライアントの `IssueTargets` と `CreateIssue` を足す。テストのサーバーで、型の往復と誤りの区別を確かめる
- [x] 3.2 `/v1/issue-targets` を作る。全プロジェクトと担当の端末が出ること、ラベル名が空の種別が除かれること、`Reload` のあとに新しいプロジェクトが出ることをテストする
- [x] 3.3 `/v1/issues` を作る。spec `inbox-api` の「issueの作成の前の確認」の誤りと、`forge` の誤り、作ったあとにキャッシュを捨てて読み直しを促すことを、失敗するテストから確かめる
- [x] 3.4 `inboxAPI` で、受信箱のトークンのクライアントをフォージの種別ごとに `Issues` として渡す

## 4. WUI（`web`）

- [x] 4.1 `/api/issue-targets` と `/api/issues` を中継する。`Origin` が無いか違うPOSTが403、JSONでないPOSTが415になり、受信箱のサービスに渡らないことをテストする
- [x] 4.2 `NewIssue` の部品を一覧の上に足す。vitestで次を確かめる。ボタンでダイアログが開くこと。開始ラベルの初期値がreadyであること。タイトルが空では送れないこと。誤りがダイアログに出て入力が残ること。作れたらダイアログが閉じ、フォージへのリンク付きの知らせが出ること
- [x] 4.3 `make verify` を通す

## 5. 実物での確認

- [ ] 5.1 V33：GitLabとGitHubで、ラベル付きで作ったissueに、作った人のラベルのイベントが残るかを確かめる。残らなければ、design D3の2段の形に替える
- [ ] 5.2 V34：usknasの常駐のサービスのWUIから、`gh:uskanda/switchboard-sandbox` に `agent:plan` でissueを作る。エージェントが計画から着手することを確かめる。`Origin` の無いPOSTが403になることも確かめる。結果をROADMAPの検証項目に書く
