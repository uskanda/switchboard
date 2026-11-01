# Tasks

## 0. 土台

- [x] 0.1 最新のmainから新しいブランチ `add-web-project-add` を作り、この変更の成果物を移す。`git status` で成果物だけが載っていることを確かめる

## 1. 設定（`config`）

- [x] 1.1 `<inbox.project_root>` を足す。`~` の展開、相対パスを拒むこと、無いときに追加が無効になることを、失敗するテストから確かめる。`config/switchboard.example.yaml` に書く
- [x] 1.2 プロジェクトを1項目足す書き込み（design D1）を作る。次を、失敗するテストから確かめる。コメント、空行、既存の項目が1文字も変わらないこと。項目の字下げに合わせること。複数行のスカラーで終わる項目の後に足すこと。シンボリックリンクの先を書くこと。検証を通らなければ書かないこと。`<config.yaml.bak>` を残すこと
- [x] 1.3 読んでから書くまでに中身が変わったら `ErrConfigChanged` を返すことを、テストで書き込みの途中にファイルを書き換えて確かめる
- [x] 1.4 `<config.Watcher>`（design D2）を作る。中身が変わらない保存で読み直さないこと、検証を通らないときに前の設定を保つこと、`projects` 以外の変化を節の名前とともに報告することを、偽の時計を使うテストで確かめる

## 2. フォージとclone（`forge`、`workspace`）

- [x] 2.1 既定のブランチとラベルの名前の一覧を読む口を、GitHub、GitLab、偽のフォージに足す。`httptest` のサーバーで、要求のパスと結果をテストする
- [x] 2.2 httpsのURLとトークンでcloneする関数と、`origin` の比較を `workspace` に足す。次をテストする。ローカルのbareリポジトリからcloneできること。失敗したら作りかけのディレクトリを消すこと。httpsの形とsshの形の `origin` を同じとみなすこと。トークンがプロセスの引数に出ないことも確かめる

## 3. エージェント（`agent`）

- [x] 3.1 読み直した `projects` を受け取る口を作り、取り下げ（design D3）を実装する。消えたプロジェクトの新しい実行が始まらないこと、実行中の実行が最後まで続くことを、偽のフォージのテストで確かめる
- [x] 3.2 受け入れ（design D3）を実装する。clone、ラベルの作成、`UntrustedPolicy`、`origin` の違い、失敗したときに次の照合でやり直すことを、偽のフォージと偽のcloneでテストする。起動時の既存のプロジェクトにはラベルを作らないことも確かめる
- [x] 3.3 `switchboard agent run` で `<config.Watcher>` を動かし、`Route` を作る関数をエージェントに渡す。中継のイベントが、受け入れたプロジェクトについて扱われることをテストする

## 4. 受信箱のサービス（`inboxserve`、`inboxapi`）

- [x] 4.1 `inboxapi` に、プロジェクトの一覧と追加の型、`Code` と、クライアントの `Projects` と `AddProject` を足す。テストのサーバーで、型の往復と誤りの区別を確かめる
- [x] 4.2 `/v1/projects` のGETを作る。クローンの有無、無い状態ラベル、フォージに届かないときの扱い、追加が有効かどうかをテストする
- [x] 4.3 `/v1/projects` のPOSTを作る。spec `inbox-api` の「プロジェクトの追加の前の確認」の6つの誤りと、足される項目の中身、webhookの案内の有無を、失敗するテストから確かめる
- [x] 4.4 書き込みのあとと `<config.Watcher>` の知らせで、`Inbox` と `Instructions.Projects` を作り直し、一覧のキャッシュを捨てる。追加の直後に、指示の一覧に新しいプロジェクトが空で出ることをテストする
- [x] 4.5 `switchboard inbox serve` で、`<config.Watcher>` を動かし、設定のパスを `Server` に渡す

## 5. WUI（`web/internal/server`、`web/ui`）

- [x] 5.1 `/api/projects` のGETとPOSTを作る。POSTで、JSONでない本文と、`Origin`/`Host` の不一致に403を返し、受信箱のサービスを呼ばないことをテストする
- [x] 5.2 設定の画面にプロジェクトの節を足す。一覧と状態、クローンが無いときの案内、追加が無効なときの案内、`plan_first` の初期値がオンであることを、VitestとTesting Libraryでテストする
- [x] 5.3 追加の欄の送信を作る。成功したら一覧を取り直すこと、webhookの案内を出すこと、`Code` ごとの理由を欄の下に出すことをテストする
- [x] 5.4 幅360pxと1280pxで設定の画面を撮り、横へのスクロールが無いことを確かめる。必要なら `DESIGN.md` の設定の画面の記述を更新する

## 6. 文書

- [x] 6.1 `docs/DECISIONS.md` にADR-039を足す（design D8）。ADR-035の該当の文に、ADR-039で改めたことを書き足す
- [x] 6.2 README.mdに、WUIからのプロジェクトの追加の使い方、`<inbox.project_root>` で有効にすること、追加のあとのwebhookの登録を書く

## 7. 結合の確認

- [x] 7.1 `make verify` を通す
- [x] 7.2 手元で `switchboard inbox serve`、`switchboard agent run`、`switchboard-web` を動かし、WUIからsandboxのリポジトリを足す。再起動せずに、clone、ラベルの作成、issueの実行が始まることを確かめ、結果をPRに書く
