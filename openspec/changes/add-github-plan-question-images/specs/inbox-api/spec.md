## MODIFIED Requirements

### Requirement: 質問の画像の取得
受信箱のAPIは、issueと画像のURLを受け取り、その画像を返す口を持たなければならない（MUST）。
返してよいのは、次のどちらかのときだけとする。ほかのURLには、要求を出す前に404を返さなければならない（MUST）。
- issueがGitLabのプロジェクトのもので、URLがそのフォージのホストの、そのプロジェクトのアップロード（`/uploads/` の下）を指すとき
- issueがGitHubのプロジェクトのもので、URLが `https://github.com/user-attachments/assets/<uuid>` の形のとき
GitLabの画像は、受信箱のトークンで取らなければならない（MUST）。GitHubの画像は、`github.com` への要求にだけ受信箱のトークンを付け、転送の先にはトークンを付けてはならない（MUST NOT）。
転送の先として許すのは、GitHubの画像の置き場所のホストだけとする。`github-production-user-asset-*.s3.amazonaws.com` と `private-user-images.githubusercontent.com` である。ほかのホストへの転送には従ってはならない（MUST NOT）。
受信箱のAPIは、画像がPNG、JPEG、GIF、WebPのどれかで、5MB以下であることを確かめてから返さなければならない（MUST）。確かめられないものは返さない。

#### Scenario: プロジェクトのアップロード
- **WHEN** `gl:team/app` #12と、`https://gitlab.example.com/team/app/uploads/abc/shot.png` を渡して画像を求める
- **THEN** 受信箱のトークンで取ったPNGが返る

#### Scenario: 別のホストの画像
- **WHEN** `gl:team/app` #12と、`https://evil.example.net/x.png` を渡して画像を求める
- **THEN** 404が返り、そのホストへの要求は出ない

#### Scenario: GitHubの添付の画像
- **WHEN** `gh:owner/app` #5と、`https://github.com/user-attachments/assets/<uuid>` を渡して画像を求める
- **THEN** GitHubの画像の置き場所へ転送された先から取ったPNGが返る。転送の先への要求にトークンは付かない

#### Scenario: GitHubの添付でないURL
- **WHEN** `gh:owner/app` #5と、`https://github.com/owner/app/raw/main/x.png` を渡して画像を求める
- **THEN** 404が返り、要求は出ない

#### Scenario: 許していない転送の先
- **WHEN** `user-attachments` のURLが、`https://evil.example.net/x.png` へ転送する
- **THEN** その転送には従わず、404が返る
