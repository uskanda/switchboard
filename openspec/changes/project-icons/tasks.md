# Tasks

## 1. フォージの層

- [ ] 1.1 `forge.ImageType`、`forge.GetImage`、`forge.Avatars`を足す。種類の判定、SVGと256KiBを超える画像の拒否、ホストを限ったトークンをテストで確かめる
- [ ] 1.2 GitHubの`ProjectAvatar`を足す。オーナーのアバターを`s=64`で取り、画像のホストにトークンを付けないことをテストで確かめる
- [ ] 1.3 GitLabの`ProjectAvatar`を足す。プロジェクトのアバター、無いときのグループのアバター、別のホストにトークンを付けないことをテストで確かめる

## 2. 設定

- [ ] 2.1 `Project`に`icon`を足し、URL、ファイル、絵文字1文字を受け付け、ほかを誤りにする。テストで確かめる

## 3. inboxサービス

- [ ] 3.1 `/v1/project-icon`と`/v1/project-icons`を足す。`icon`の優先、24時間の保存、失敗の1時間の記憶、404をテストで確かめる
- [ ] 3.2 `inboxapi.Client`に`ProjectIcon`と`ProjectIcons`を足し、テストで確かめる
- [ ] 3.3 `switchboard inbox serve`で、受信箱のトークンのフォージと状態の置き場を渡す

## 4. switchboard-web

- [ ] 4.1 `/api/project-icon`と`/api/project-icons`を足す。転送、`Content-Type`、CSPが今のままであることを`server_test.go`で確かめる

## 5. Webの画面

- [ ] 5.1 `ProjectIcon`と絵文字の一覧の読み込みを足し、`IssueRef`の参照の前に置く
- [ ] 5.2 vitestで、画像、失敗したときの頭文字、絵文字、`alt=""`を確かめる

## 6. 文書

- [ ] 6.1 `docs/DECISIONS.md`にADR-047を足し、`docs/SECURITY.md`の4章に受信箱のトークンでアバターを読むことを書く
- [ ] 6.2 `DESIGN.md`の参照の項と`project-initial`、`docs/SPEC.md`、設定の例に`icon`を足す

## 7. 全体の確認

- [ ] 7.1 `make verify`が通ることを確かめる
- [ ] 7.2 `openspec validate project-icons --strict`が通ることを確かめる
